package video

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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

	// Assuming the Video struct has a UserID field of type primitive.ObjectID.
	newVideo := &Video{
		ID:          videoID,
		Title:       title,
		Description: description,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	outFile, err := os.Create(rawFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create raw file: %w", err)
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		os.Remove(rawFilePath) // Clean up the created file if copy fails.
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	_, err = s.videoCollection.InsertOne(ctx, newVideo)
	if err != nil {
		os.Remove(rawFilePath) // Clean up the created file if database insert fails.
		return nil, err
	}
	go s.startTranscoding(newVideo.ID, rawFilePath)

	return newVideo, nil
}

func (s *VideoService) startTranscoding(videoID primitive.ObjectID, rawFile string) {
	ctx := context.Background()

	//update video status to processing
	_, err := s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, bson.M{"$set": bson.M{"status": StatusProcessing}})
	if err != nil {
		log.Printf("Error updating video status to processing: %v", err)
		return
	}

	outputDir := fmt.Sprintf("storage/processed/%s", videoID.Hex())
	os.MkdirAll(outputDir, os.ModePerm)
	hlsPlaylist := fmt.Sprintf("%s/playlist.m3u8", outputDir)

	cmd := exec.Command("ffmpeg", "-i", rawFile, "-c:v", "libx264", "-c:a", "aac", "-hls_time", "10", "-hls_list_size", "0", "-f", "hls", hlsPlaylist)

	if err := cmd.Run(); err != nil {
		log.Printf("Error transcoding video: %v", err)
		s.videoCollection.UpdateOne(context.Background(), bson.M{"_id": videoID}, bson.M{"$set": bson.M{"status": StatusFailed}})
		return
	}

	update := bson.M{
		"$set": bson.M{
			"status":  StatusCompleted,
			"hlsPath": hlsPlaylist,
		},
	}

	//update video status to completed
	_, err = s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, update)
	if err != nil {
		log.Printf("Error updating video status to completed: %v", err)
		return
	}

	log.Printf("Video transcoded successfully: %s", videoID.Hex())
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

	// Delete the raw video file.
	rawFilePath := fmt.Sprintf("storage/uploads/%s.mp4", video.ID.Hex())
	if err := os.Remove(rawFilePath); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to delete raw video file %s: %v", rawFilePath, err)
	}

	// Delete the processed HLS files directory.
	processedDir := fmt.Sprintf("storage/processed/%s", video.ID.Hex())
	if err := os.RemoveAll(processedDir); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to delete processed video directory %s: %v", processedDir, err)
	}

	// Delete the video record from the database.
	_, err = s.videoCollection.DeleteOne(ctx, bson.M{"_id": id})
	if err != nil {
		return fmt.Errorf("failed to delete video record: %w", err)
	}

	return nil
}


