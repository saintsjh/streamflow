package livestream

import (
	"fmt"
	"os/exec"
	"strings"
)

// FFmpegService handles FFmpeg operations
type FFmpegService struct {
	ffmpegPath string
}

// NewFFmpegService creates a new FFmpeg service
func NewFFmpegService() *FFmpegService {
	return &FFmpegService{
		ffmpegPath: "ffmpeg", // Assumes ffmpeg is in PATH
	}
}

// CheckFFmpegAvailable checks if FFmpeg is installed and available
func (f *FFmpegService) CheckFFmpegAvailable() error {
	cmd := exec.Command(f.ffmpegPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("ffmpeg not found: %w", err)
	}

	// Check if output contains FFmpeg version info
	if !strings.Contains(string(output), "ffmpeg version") {
		return fmt.Errorf("ffmpeg not properly installed")
	}

	return nil
}

// ConvertVideo converts a video file to different format/resolution
func (f *FFmpegService) ConvertVideo(inputPath, outputPath string, options map[string]string) error {
	args := []string{"-i", inputPath}

	// Add custom options
	for key, value := range options {
		args = append(args, key, value)
	}

	args = append(args, outputPath)

	cmd := exec.Command(f.ffmpegPath, args...)
	cmd.Stderr = nil // FFmpeg outputs progress to stderr

	return cmd.Run()
}

// ExtractThumbnail extracts a thumbnail from video at specified time
func (f *FFmpegService) ExtractThumbnail(videoPath, thumbnailPath string, timeOffset string) error {
	args := []string{
		"-i", videoPath,
		"-ss", timeOffset, // Time offset (e.g., "00:00:10")
		"-vframes", "1",
		"-q:v", "2", // High quality
		thumbnailPath,
	}

	cmd := exec.Command(f.ffmpegPath, args...)
	return cmd.Run()
}

// GetVideoInfo gets basic information about a video file
func (f *FFmpegService) GetVideoInfo(videoPath string) (string, error) {
	args := []string{
		"-i", videoPath,
		"-hide_banner",
		"-f", "null",
		"-",
	}

	cmd := exec.Command(f.ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to get video info: %w", err)
	}

	return string(output), nil
}

// TestFFmpegConnection tests if FFmpeg is accessible and working
func (f *FFmpegService) TestFFmpegConnection() (string, error) {
	cmd := exec.Command(f.ffmpegPath, "-version")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ffmpeg test failed: %w", err)
	}

	// Return first line of version info
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0]), nil
	}

	return "FFmpeg found but no version info", nil
}
