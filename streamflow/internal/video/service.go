package video

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UpdateVideoRequest defines the structure for a request to update a video.
type UpdateVideoRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type VideoService struct {
	videoCollection *mongo.Collection
}

func NewVideoService(db *mongo.Database) *VideoService {
	return &VideoService{
		videoCollection: db.Collection("videos"),
	}
}

// CreateVideo now accepts a primitive.ObjectID for the userID and includes it in the new video document.
func (s *VideoService) CreateVideo(ctx context.Context, file io.Reader, title, description string, userID primitive.ObjectID) (*Video, error) {
	videoID := primitive.NewObjectID()
	rawFilePath := fmt.Sprintf("storage/uploads/%s.mp4", videoID.Hex())
	thumbnailPath := fmt.Sprintf("storage/cache/thumbnails/%s.jpg", videoID.Hex())

	// Create upload directory if it doesn't exist
	uploadDir := filepath.Dir(rawFilePath)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Create new video document
	newVideo := &Video{
		ID:          videoID,
		Title:       title,
		Description: description,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		UserID:      userID,
		FilePath:    rawFilePath,
	}

	// Save the uploaded file
	outFile, err := os.Create(rawFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create raw file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		CleanupFailedUpload(rawFilePath)
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	// Detect corrupt video file
	if err := DetectCorruptVideo(rawFilePath); err != nil {
		CleanupFailedUpload(rawFilePath)
		return nil, fmt.Errorf("video file validation failed: %w", err)
	}

	// Extract video metadata
	metadata, err := ExtractVideoMetadata(rawFilePath)
	if err != nil {
		CleanupFailedUpload(rawFilePath)
		return nil, fmt.Errorf("failed to extract video metadata: %w", err)
	}

	// Validate extracted metadata
	if err := ValidateVideoMetadata(metadata); err != nil {
		CleanupFailedUpload(rawFilePath)
		return nil, fmt.Errorf("video metadata validation failed: %w", err)
	}

	// Generate thumbnail
	if err := GenerateThumbnail(rawFilePath, thumbnailPath); err != nil {
		log.Printf("Failed to generate thumbnail for video %s: %v", videoID.Hex(), err)
		// Don't fail the upload if thumbnail generation fails
	} else {
		newVideo.ThumbnailPath = thumbnailPath
	}

	// Store metadata in video document
	newVideo.Metadata = *metadata

	// Insert video document into database
	_, err = s.videoCollection.InsertOne(ctx, newVideo)
	if err != nil {
		CleanupFailedUpload(rawFilePath, thumbnailPath)
		return nil, fmt.Errorf("failed to save video to database: %w", err)
	}

	// Start transcoding in background
	go s.startTranscoding(newVideo.ID, rawFilePath)

	return newVideo, nil
}

func (s *VideoService) startTranscoding(videoID primitive.ObjectID, rawFile string) {
	ctx := context.Background()

	// Update video status to processing
	_, err := s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, bson.M{"$set": bson.M{"status": StatusProcessing}})
	if err != nil {
		log.Printf("Error updating video status to processing: %v", err)
		return
	}

	outputDir := fmt.Sprintf("storage/processed/%s", videoID.Hex())
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Error creating output directory: %v", err)
		s.updateVideoStatus(ctx, videoID, StatusFailed, "Failed to create output directory")
		return
	}

	hlsPlaylist := fmt.Sprintf("%s/playlist.m3u8", outputDir)

	// Enhanced ffmpeg command with multiple quality levels
	cmd := exec.Command("ffmpeg",
		"-i", rawFile,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-hls_time", "10",
		"-hls_list_size", "0",
		"-hls_segment_filename", fmt.Sprintf("%s/segment_%%03d.ts", outputDir),
		"-f", "hls",
		hlsPlaylist)

	if err := cmd.Run(); err != nil {
		log.Printf("Error transcoding video: %v", err)
		s.updateVideoStatus(ctx, videoID, StatusFailed, fmt.Sprintf("Transcoding failed: %v", err))
		return
	}

	// Update video with HLS path and completed status
	update := bson.M{
		"$set": bson.M{
			"status":   StatusCompleted,
			"hlsPath":  hlsPlaylist,
			"updatedAt": time.Now(),
		},
	}

	_, err = s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, update)
	if err != nil {
		log.Printf("Error updating video status to completed: %v", err)
		return
	}

	log.Printf("Video transcoded successfully: %s", videoID.Hex())
}

// updateVideoStatus is a helper method to update video status with error message
func (s *VideoService) updateVideoStatus(ctx context.Context, videoID primitive.ObjectID, status VideoStatus, errorMsg string) {
	update := bson.M{
		"$set": bson.M{
			"status":   status,
			"error":    errorMsg,
			"updatedAt": time.Now(),
		},
	}

	_, err := s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, update)
	if err != nil {
		log.Printf("Error updating video status: %v", err)
	}
}

// GetVideoByID retrieves a single video by its ID.
func (s *VideoService) GetVideoByID(ctx context.Context, id primitive.ObjectID) (*Video, error) {
	var video Video
	err := s.videoCollection.FindOne(ctx, bson.M{"_id": id}).Decode(&video)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("video not found")
		}
		return nil, err
	}
	return &video, nil
}

// ListVideos retrieves a paginated list of videos.
func (s *VideoService) ListVideos(ctx context.Context, page, limit int) ([]*Video, error) {
	findOptions := options.Find()
	findOptions.SetSkip(int64((page - 1) * limit))
	findOptions.SetLimit(int64(limit))
	findOptions.SetSort(bson.D{{Key: "createdAt", Value: -1}}) // Sort by newest first

	cursor, err := s.videoCollection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var videos []*Video
	if err = cursor.All(ctx, &videos); err != nil {
		return nil, err
	}
	return videos, nil
}

// UpdateVideo updates a video's metadata based on the provided request.
func (s *VideoService) UpdateVideo(ctx context.Context, id primitive.ObjectID, req UpdateVideoRequest) (*Video, error) {
	updateFields := bson.M{}
	if req.Title != "" {
		updateFields["title"] = req.Title
	}
	if req.Description != "" {
		updateFields["description"] = req.Description
	}

	if len(updateFields) == 0 {
		return s.GetVideoByID(ctx, id) // Nothing to update, return current data.
	}

	updateFields["updatedAt"] = time.Now()
	update := bson.M{"$set": updateFields}

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	result := s.videoCollection.FindOneAndUpdate(ctx, bson.M{"_id": id}, update, opts)
	if result.Err() != nil {
		return nil, result.Err()
	}

	var updatedVideo Video
	if err := result.Decode(&updatedVideo); err != nil {
		return nil, err
	}
	return &updatedVideo, nil
}

// DeleteVideo removes a video record and its associated files from storage.
func (s *VideoService) DeleteVideo(ctx context.Context, id primitive.ObjectID) error {
	video, err := s.GetVideoByID(ctx, id)
	if err != nil {
		if err.Error() == "video not found" {
			return nil // Video doesn't exist, so we consider it deleted.
		}
		return err
	}

	// Delete the raw video file
	if video.FilePath != "" {
		if err := os.Remove(video.FilePath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to delete raw video file %s: %v", video.FilePath, err)
		}
	}

	// Delete the thumbnail file
	if video.ThumbnailPath != "" {
		if err := os.Remove(video.ThumbnailPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to delete thumbnail file %s: %v", video.ThumbnailPath, err)
		}
	}

	// Delete the processed HLS files directory
	if video.HLSPath != "" {
		processedDir := filepath.Dir(video.HLSPath)
		if err := os.RemoveAll(processedDir); err != nil && !os.IsNotExist(err) {
			log.Printf("Failed to delete processed video directory %s: %v", processedDir, err)
		}
	}

	// Delete the video record from the database
	_, err = s.videoCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete video record: %w", err)
	}

	return nil
}

// IncrementViewCount increments the view count for a video when it's watched
func (s *VideoService) IncrementViewCount(ctx context.Context, videoID primitive.ObjectID) error {
	update := bson.M{"$inc": bson.M{"view_count": 1}}
	
	result, err := s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, update)
	if err != nil {
		return fmt.Errorf("failed to increment view count: %w", err)
	}
	
	if result.MatchedCount == 0 {
		return fmt.Errorf("video not found")
	}
	
	return nil
}

// GetPopularVideos returns videos ordered by view count (most viewed first)
func (s *VideoService) GetPopularVideos(ctx context.Context, limit int) ([]*Video, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "view_count", Value: -1}}).
		SetLimit(int64(limit))
	
	cursor, err := s.videoCollection.Find(ctx, bson.M{"status": StatusCompleted}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	var videos []*Video
	if err = cursor.All(ctx, &videos); err != nil {
		return nil, err
	}
	return videos, nil
}

// GetTrendingVideos returns recently uploaded videos with high view counts
func (s *VideoService) GetTrendingVideos(ctx context.Context, limit int, daysBack int) ([]*Video, error) {
	// Calculate date threshold (e.g., videos from last 7 days)
	threshold := time.Now().AddDate(0, 0, -daysBack)
	
	opts := options.Find().
		SetSort(bson.D{
			{Key: "view_count", Value: -1},
			{Key: "created_at", Value: -1},
		}).
		SetLimit(int64(limit))
	
	filter := bson.M{
		"status": StatusCompleted,
		"created_at": bson.M{"$gte": threshold},
	}
	
	cursor, err := s.videoCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	
	var videos []*Video
	if err = cursor.All(ctx, &videos); err != nil {
		return nil, err
	}
	return videos, nil
}


