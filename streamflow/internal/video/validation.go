package video

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	MaxFileSize = 500 * 1024 * 1024 // 500MB
	MaxDuration = 3600               // 1 hour in seconds
)

var AllowedVideoTypes = map[string]bool{
	"video/mp4":  true,
	"video/avi":  true,
	"video/mov":  true,
	"video/mkv":  true,
	"video/webm": true,
}

type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateVideoFile performs comprehensive validation of uploaded video files
func ValidateVideoFile(file *multipart.FileHeader) error {
	// Check file size
	if file.Size > MaxFileSize {
		return ValidationError{
			Field:   "file",
			Message: fmt.Sprintf("File size %d bytes exceeds maximum allowed size of %d bytes", file.Size, MaxFileSize),
		}
	}

	// Check file type
	contentType := file.Header.Get("Content-Type")
	if !AllowedVideoTypes[contentType] {
		return ValidationError{
			Field:   "file",
			Message: fmt.Sprintf("File type %s is not allowed. Allowed types: %v", contentType, getAllowedTypes()),
		}
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := []string{".mp4", ".avi", ".mov", ".mkv", ".webm"}
	allowed := false
	for _, allowedExt := range allowedExts {
		if ext == allowedExt {
			allowed = true
			break
		}
	}
	if !allowed {
		return ValidationError{
			Field:   "file",
			Message: fmt.Sprintf("File extension %s is not allowed. Allowed extensions: %v", ext, allowedExts),
		}
	}

	return nil
}

// ExtractVideoMetadata extracts video metadata using ffprobe
func ExtractVideoMetadata(filePath string) (*VideoMetadata, error) {
	// Use ffprobe to get video information
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to extract metadata: %w", err)
	}

	var result struct {
		Format  map[string]interface{} `json:"format"`
		Streams []struct {
			CodecType string  `json:"codec_type"`
			CodecName string  `json:"codec_name"`
			Width     int     `json:"width,omitempty"`
			Height    int     `json:"height,omitempty"`
			Duration  string  `json:"duration,omitempty"`
			BitRate   string  `json:"bit_rate,omitempty"`
			RFrameRate string `json:"r_frame_rate,omitempty"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	metadata := &VideoMetadata{}

	// Get file size
	if fileInfo, err := os.Stat(filePath); err == nil {
		metadata.FileSize = fileInfo.Size()
	}

	// Extract format information
	if format, ok := result.Format["duration"]; ok {
		if durationStr, ok := format.(string); ok {
			if duration, err := strconv.ParseFloat(durationStr, 64); err == nil {
				metadata.Duration = duration
			}
		}
	}

	// Extract stream information
	for _, stream := range result.Streams {
		if stream.CodecType == "video" {
			metadata.Width = stream.Width
			metadata.Height = stream.Height
			metadata.Codec = stream.CodecName
			
			// Parse frame rate
			if stream.RFrameRate != "" {
				parts := strings.Split(stream.RFrameRate, "/")
				if len(parts) == 2 {
					if num, err := strconv.ParseFloat(parts[0], 64); err == nil {
						if den, err := strconv.ParseFloat(parts[1], 64); err == nil && den > 0 {
							metadata.FrameRate = num / den
						}
					}
				}
			}
		} else if stream.CodecType == "audio" {
			metadata.AudioCodec = stream.CodecName
		}
	}

	// Calculate bitrate
	if format, ok := result.Format["bit_rate"]; ok {
		if bitrateStr, ok := format.(string); ok {
			if bitrate, err := strconv.Atoi(bitrateStr); err == nil {
				metadata.Bitrate = bitrate / 1000 // Convert to kbps
			}
		}
	}

	return metadata, nil
}

// DetectCorruptVideo checks if the video file is corrupted
func DetectCorruptVideo(filePath string) error {
	// Use ffprobe to check if video can be read
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type",
		"-of", "csv=p=0",
		filePath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("video file appears to be corrupted or unreadable: %w", err)
	}

	return nil
}

// GenerateThumbnail creates a thumbnail from the video file
func GenerateThumbnail(videoPath, thumbnailPath string) error {
	// Create thumbnail directory if it doesn't exist
	thumbnailDir := filepath.Dir(thumbnailPath)
	if err := os.MkdirAll(thumbnailDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Use ffmpeg to generate thumbnail at 5 seconds into the video
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-ss", "00:00:05", // Seek to 5 seconds
		"-vframes", "1",   // Extract 1 frame
		"-vf", "scale=320:240", // Scale to 320x240
		"-y", // Overwrite output file
		thumbnailPath)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	return nil
}

// CleanupFailedUpload removes files created during failed upload process
func CleanupFailedUpload(filePaths ...string) {
	for _, path := range filePaths {
		if path != "" {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				log.Printf("Failed to cleanup file %s: %v", path, err)
			}
		}
	}
}

// ValidateVideoMetadata checks if extracted metadata is within acceptable ranges
func ValidateVideoMetadata(metadata *VideoMetadata) error {
	if metadata.Duration > MaxDuration {
		return ValidationError{
			Field:   "duration",
			Message: fmt.Sprintf("Video duration %.2f seconds exceeds maximum allowed duration of %d seconds", metadata.Duration, MaxDuration),
		}
	}

	if metadata.Width <= 0 || metadata.Height <= 0 {
		return ValidationError{
			Field:   "resolution",
			Message: "Invalid video resolution",
		}
	}

	if metadata.FileSize <= 0 {
		return ValidationError{
			Field:   "file_size",
			Message: "Invalid file size",
		}
	}

	return nil
}

func getAllowedTypes() []string {
	types := make([]string, 0, len(AllowedVideoTypes))
	for t := range AllowedVideoTypes {
		types = append(types, t)
	}
	return types
} 