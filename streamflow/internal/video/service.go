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
)

type VideoService struct {
	videoCollection *mongo.Collection
}

func NewVideoService(db *mongo.Database) *VideoService {
	return &VideoService{
		videoCollection: db.Collection("videos"),
	}
}

func (s *VideoService) CreateVideo(ctx context.Context, file io.Reader, title, description, uploaderID string) (*Video, error) {
	videoID := primitive.NewObjectID()
	rawFilePath := fmt.Sprintf("storage/uploads/%s.mp4", videoID.Hex())

	newVideo := &Video{
		ID:          videoID,
		Title:       title,
		Description: description,
		UploaderID:  uploaderID,
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
		return nil, fmt.Errorf("failed to save file: %w", err)
	}

	_, err = s.videoCollection.InsertOne(ctx, newVideo)
	if err != nil {
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


