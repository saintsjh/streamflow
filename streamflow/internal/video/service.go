package video

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bytes"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UpdateVideoRequest defines the structure for a request to update a video.
type UpdateVideoRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type VideoService struct {
	videoCollection *mongo.Collection
	fs              *gridfs.Bucket
}

func NewVideoService(db *mongo.Database) *VideoService {
	fs, err := gridfs.NewBucket(db)
	if err != nil {
		log.Fatalf("Failed to create GridFS bucket: %v", err)
	}

	return &VideoService{
		videoCollection: db.Collection("videos"),
		fs:              fs,
	}
}

// CreateVideo now accepts a primitive.ObjectID for the userID and includes it in the new video document.
func (s *VideoService) CreateVideo(ctx context.Context, file io.Reader, title, description string, userID primitive.ObjectID, thumbnail io.Reader) (*Video, error) {
	log.Printf("CreateVideo called for user %s with title '%s'", userID.Hex(), title)
	videoID := primitive.NewObjectID()
	log.Printf("Generated new video ID: %s", videoID.Hex())
	newVideo := &Video{
		ID:          videoID,
		Title:       title,
		Description: description,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		UserID:      userID,
		FilePath:    fmt.Sprintf("%s.mp4", videoID.Hex()), // GridFS filename
	}

	// TeeReader to write to both GridFS and a temporary local file
	tempFilePath := fmt.Sprintf("storage/uploads/%s_temp.mp4", videoID.Hex())
	if err := os.MkdirAll(filepath.Dir(tempFilePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create upload directory: %w", err)
	}

	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tempFile.Close()

	// Setup GridFS upload stream
	uploadStream, err := s.fs.OpenUploadStreamWithID(videoID, newVideo.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open upload stream: %w", err)
	}
	defer uploadStream.Close()

	// Use a TeeReader to write to both GridFS and the local file simultaneously
	teeReader := io.TeeReader(file, tempFile)

	if _, err := io.Copy(uploadStream, teeReader); err != nil {
		CleanupFailedUpload(tempFilePath)
		return nil, fmt.Errorf("failed to save file to GridFS and temp file: %w", err)
	}
	log.Println("Finished writing video to GridFS and temp file")

	// Detect corrupt video file from the temporary file
	log.Println("Detecting corrupt video...")
	if err := DetectCorruptVideo(tempFilePath); err != nil {
		CleanupFailedUpload(tempFilePath)
		return nil, fmt.Errorf("video file validation failed: %w", err)
	}

	// Extract video metadata from the temporary file
	log.Println("Extracting video metadata...")
	metadata, err := ExtractVideoMetadata(tempFilePath)
	if err != nil {
		CleanupFailedUpload(tempFilePath)
		return nil, fmt.Errorf("failed to extract video metadata: %w", err)
	}

	// Validate extracted metadata
	log.Println("Validating video metadata...")
	if err := ValidateVideoMetadata(metadata); err != nil {
		CleanupFailedUpload(tempFilePath)
		return nil, fmt.Errorf("video metadata validation failed: %w", err)
	}

	// Handle thumbnail
	var thumbnailGridFSID primitive.ObjectID
	if thumbnail != nil {
		// Upload provided thumbnail
		var err error
		thumbnailGridFSID, err = s.uploadThumbnail(thumbnail, videoID)
		if err != nil {
			log.Printf("Failed to upload thumbnail for video %s: %v", videoID.Hex(), err)
		}
	} else {
		// Generate thumbnail from video
		var err error
		thumbnailGridFSID, err = s.generateAndUploadThumbnail(tempFilePath, videoID)
		if err != nil {
			log.Printf("Failed to generate thumbnail for video %s: %v", videoID.Hex(), err)
		}
	}

	if thumbnailGridFSID != primitive.NilObjectID {
		newVideo.ThumbnailPath = thumbnailGridFSID.Hex() // Store GridFS ID
	}

	// Store metadata in video document
	newVideo.Metadata = *metadata

	// Insert video document into database
	_, err = s.videoCollection.InsertOne(ctx, newVideo)
	if err != nil {
		CleanupFailedUpload(tempFilePath)
		return nil, fmt.Errorf("failed to save video to database: %w", err)
	}

	// Start transcoding in the background using the temporary file
	go s.startTranscoding(videoID, tempFilePath)

	return newVideo, nil
}

func (s *VideoService) generateAndUploadThumbnail(videoPath string, videoID primitive.ObjectID) (primitive.ObjectID, error) {
	thumbnailID := primitive.NewObjectID()
	thumbnailPath := fmt.Sprintf("storage/cache/thumbnails/%s.jpg", videoID.Hex())

	// Create thumbnail directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(thumbnailPath), 0755); err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Use ffmpeg to generate thumbnail
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-ss", "00:00:05",
		"-vframes", "1",
		"-vf", "scale=320:-1",
		"-y",
		thumbnailPath)

	if err := cmd.Run(); err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to generate thumbnail: %w", err)
	}

	// Upload to GridFS
	file, err := os.Open(thumbnailPath)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to open thumbnail file for upload: %w", err)
	}
	defer file.Close()

	uploadStream, err := s.fs.OpenUploadStreamWithID(thumbnailID, fmt.Sprintf("%s_thumbnail.jpg", videoID.Hex()))
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to open GridFS upload stream for thumbnail: %w", err)
	}
	defer uploadStream.Close()

	if _, err := io.Copy(uploadStream, file); err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to upload thumbnail to GridFS: %w", err)
	}

	// Clean up local thumbnail file
	if err := os.Remove(thumbnailPath); err != nil {
		log.Printf("Failed to remove temporary thumbnail file: %v", err)
	}

	return thumbnailID, nil
}

func (s *VideoService) uploadThumbnail(thumbnail io.Reader, videoID primitive.ObjectID) (primitive.ObjectID, error) {
	thumbnailID := primitive.NewObjectID()

	if thumbnail == nil {
		return primitive.NilObjectID, fmt.Errorf("thumbnail reader is nil")
	}

	uploadStream, err := s.fs.OpenUploadStreamWithID(thumbnailID, fmt.Sprintf("%s_thumbnail.jpg", videoID.Hex()))
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to open GridFS upload stream for thumbnail: %w", err)
	}

	_, err = io.Copy(uploadStream, thumbnail)
	if err != nil {
		uploadStream.Close()
		return primitive.NilObjectID, fmt.Errorf("failed to upload thumbnail to GridFS: %w", err)
	}

	if err := uploadStream.Close(); err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to close thumbnail upload stream: %w", err)
	}

	return thumbnailID, nil
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

	hlsPlaylistPath := filepath.Join(outputDir, "playlist.m3u8")

	// Use the segment muxer to create HLS segments in a temporary directory
	cmd := exec.Command("ffmpeg",
		"-i", rawFile,
		"-c:v", "libx264",
		"-c:a", "aac",
		"-f", "segment",
		"-segment_time", "10",
		"-segment_list", hlsPlaylistPath,
		"-segment_format", "mpegts",
		filepath.Join(outputDir, "segment%03d.ts"),
	)

	// Capture stderr for better error logging
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Printf("Error transcoding video: %v, stderr: %s", err, stderr.String())
		s.updateVideoStatus(ctx, videoID, StatusFailed, fmt.Sprintf("Transcoding failed: %v - %s", err, stderr.String()))
		return
	}

	// After transcoding, upload the playlist and segments to GridFS
	if err := uploadHLSToGridFS(s.fs, outputDir, videoID); err != nil {
		log.Printf("Failed to upload HLS files to GridFS: %v", err)
		s.updateVideoStatus(ctx, videoID, StatusFailed, "Failed to upload HLS files")
		return
	}

	// Clean up the temporary directory
	if err := os.RemoveAll(outputDir); err != nil {
		log.Printf("Failed to remove temporary processing directory: %v", err)
	}

	// Clean up the temporary raw file
	if err := os.Remove(rawFile); err != nil {
		log.Printf("Failed to remove temporary raw file: %v", err)
	}

	// Update video with HLS path and completed status
	update := bson.M{
		"$set": bson.M{
			"status":     StatusCompleted,
			"hls_path":   fmt.Sprintf("%s/playlist.m3u8", videoID.Hex()), // GridFS path
			"updated_at": time.Now(),
		},
	}

	_, err = s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, update)
	if err != nil {
		log.Printf("Error updating video status to completed: %v", err)
		return
	}

	log.Printf("Video transcoded successfully: %s", videoID.Hex())
}

// uploadHLSToGridFS reads all HLS files from a directory and uploads them to GridFS.
func uploadHLSToGridFS(fs *gridfs.Bucket, dirPath string, videoID primitive.ObjectID) error {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("could not read processing directory: %w", err)
	}

	var uploadErrors []string
	playlistUploaded := false

	for _, file := range files {
		filePath := filepath.Join(dirPath, file.Name())
		gridFSFilename := fmt.Sprintf("%s/%s", videoID.Hex(), file.Name())

		fileReader, err := os.Open(filePath)
		if err != nil {
			log.Printf("Could not open file %s for GridFS upload: %v", filePath, err)
			uploadErrors = append(uploadErrors, fmt.Sprintf("failed to open %s: %v", file.Name(), err))
			continue
		}

		uploadStream, err := fs.OpenUploadStream(gridFSFilename)
		if err != nil {
			fileReader.Close()
			log.Printf("Could not open GridFS upload stream for %s: %v", gridFSFilename, err)
			uploadErrors = append(uploadErrors, fmt.Sprintf("failed to create upload stream for %s: %v", file.Name(), err))
			continue
		}

		_, copyErr := io.Copy(uploadStream, fileReader)
		fileReader.Close()
		uploadStream.Close()

		if copyErr != nil {
			log.Printf("Could not copy file %s to GridFS: %v", filePath, copyErr)
			uploadErrors = append(uploadErrors, fmt.Sprintf("failed to upload %s: %v", file.Name(), copyErr))
		} else {
			log.Printf("Successfully uploaded %s to GridFS", gridFSFilename)
			if file.Name() == "playlist.m3u8" {
				playlistUploaded = true
			}
		}
	}

	// Critical error: playlist.m3u8 must be uploaded for streaming to work
	if !playlistUploaded {
		return fmt.Errorf("critical error: playlist.m3u8 was not uploaded to GridFS")
	}

	// If we have upload errors, log them but don't fail if playlist is uploaded
	if len(uploadErrors) > 0 {
		log.Printf("Some files failed to upload to GridFS: %v", uploadErrors)
	}

	return nil
}

// updateVideoStatus is a helper method to update video status with error message
func (s *VideoService) updateVideoStatus(ctx context.Context, videoID primitive.ObjectID, status VideoStatus, errorMsg string) {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"error":      errorMsg,
			"updated_at": time.Now(),
		},
	}

	_, err := s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, update)
	if err != nil {
		log.Printf("Error updating video status: %v", err)
	}
}

// UpdateVideoStatus updates a video's status (public method for manual status updates)
func (s *VideoService) UpdateVideoStatus(ctx context.Context, videoID primitive.ObjectID, status VideoStatus) error {
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	result, err := s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, update)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

// GridFSHLSWriter implements io.Writer to upload HLS segments to GridFS
type GridFSHLSWriter struct {
	fs        *gridfs.Bucket
	videoID   primitive.ObjectID
	outputDir string
	wg        *sync.WaitGroup
}

func (w *GridFSHLSWriter) Write(p []byte) (int, error) {
	segmentName := strings.TrimSpace(string(p))
	segmentPath := filepath.Join(w.outputDir, segmentName)
	gridfsFilename := fmt.Sprintf("%s/%s", w.videoID.Hex(), segmentName)

	w.wg.Add(1)
	go func() {
		defer w.wg.Done()

		file, err := os.Open(segmentPath)
		if err != nil {
			log.Printf("Error opening segment file for upload: %v", err)
			return
		}
		defer file.Close()

		uploadStream, err := w.fs.OpenUploadStream(gridfsFilename)
		if err != nil {
			log.Printf("Error opening GridFS upload stream for segment: %v", err)
			return
		}
		defer uploadStream.Close()

		if _, err := io.Copy(uploadStream, file); err != nil {
			log.Printf("Error uploading segment to GridFS: %v", err)
		}

		// Clean up the local segment file after upload
		os.Remove(segmentPath)
	}()

	return len(p), nil
}

// DownloadFromGridFS downloads a file from GridFS by its filename
func (s *VideoService) DownloadFromGridFS(ctx context.Context, filename string) (*gridfs.DownloadStream, error) {
	downloadStream, err := s.fs.OpenDownloadStreamByName(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open download stream for %s: %w", filename, err)
	}
	return downloadStream, nil
}

func (s *VideoService) DownloadFromGridFSByID(ctx context.Context, id primitive.ObjectID) (*gridfs.DownloadStream, error) {
	downloadStream, err := s.fs.OpenDownloadStream(id)
	if err != nil {
		return nil, fmt.Errorf("failed to open download stream for id %s: %w", id.Hex(), err)
	}
	return downloadStream, nil
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

	var videos []*Video = []*Video{}
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

	updateFields["updated_at"] = time.Now()
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

	// Delete the original video file from GridFS
	if fileID, err := primitive.ObjectIDFromHex(video.ID.Hex()); err == nil {
		if err := s.fs.Delete(fileID); err != nil {
			log.Printf("Failed to delete original video file from GridFS %s: %v", video.ID.Hex(), err)
		}
	}

	// Delete the thumbnail file from GridFS
	if video.ThumbnailPath != "" {
		if thumbnailID, err := primitive.ObjectIDFromHex(video.ThumbnailPath); err == nil {
			if err := s.fs.Delete(thumbnailID); err != nil {
				log.Printf("Failed to delete thumbnail file from GridFS %s: %v", video.ThumbnailPath, err)
			}
		}
	}

	// Delete HLS segments and playlist from GridFS
	if video.HLSPath != "" {
		// Find all files related to the videoID in GridFS and delete them
		prefix := fmt.Sprintf("%s/", video.ID.Hex())
		cursor, err := s.fs.Find(bson.M{"filename": bson.M{"$regex": prefix}})
		if err == nil {
			for cursor.Next(ctx) {
				var file bson.M
				if err := cursor.Decode(&file); err == nil {
					fileID := file["_id"].(primitive.ObjectID)
					if err := s.fs.Delete(fileID); err != nil {
						log.Printf("Failed to delete HLS file %s from GridFS: %v", file["filename"], err)
					}
				}
			}
			cursor.Close(ctx)
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

// ReprocessFailedVideos finds videos that are marked as COMPLETED but have no HLS path
// and attempts to upload their existing processed files to GridFS
func (s *VideoService) ReprocessFailedVideos(ctx context.Context) error {
	// Find videos that are COMPLETED but have empty HLS paths
	filter := bson.M{
		"status": StatusCompleted,
		"$or": []bson.M{
			{"hls_path": ""},
			{"hls_path": bson.M{"$exists": false}},
		},
	}

	cursor, err := s.videoCollection.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to find videos for reprocessing: %w", err)
	}
	defer cursor.Close(ctx)

	var videos []Video
	if err = cursor.All(ctx, &videos); err != nil {
		return fmt.Errorf("failed to decode videos for reprocessing: %w", err)
	}

	log.Printf("Found %d videos needing HLS file upload to GridFS", len(videos))

	for _, video := range videos {
		log.Printf("Reprocessing video %s (%s)", video.ID.Hex(), video.Title)
		
		// Check if local processed files exist
		processedDir := fmt.Sprintf("storage/processed/%s", video.ID.Hex())
		if _, err := os.Stat(processedDir); os.IsNotExist(err) {
			log.Printf("No processed files found for video %s, skipping", video.ID.Hex())
			continue
		}

		// Upload HLS files to GridFS
		if err := uploadHLSToGridFS(s.fs, processedDir, video.ID); err != nil {
			log.Printf("Failed to upload HLS files for video %s: %v", video.ID.Hex(), err)
			continue
		}

		// Update video with HLS path
		update := bson.M{
			"$set": bson.M{
				"hls_path":   fmt.Sprintf("%s/playlist.m3u8", video.ID.Hex()),
				"updated_at": time.Now(),
			},
		}

		_, err = s.videoCollection.UpdateOne(ctx, bson.M{"_id": video.ID}, update)
		if err != nil {
			log.Printf("Failed to update video %s with HLS path: %v", video.ID.Hex(), err)
			continue
		}

		// Clean up local files after successful upload
		if err := os.RemoveAll(processedDir); err != nil {
			log.Printf("Failed to clean up processed directory for video %s: %v", video.ID.Hex(), err)
		}

		log.Printf("Successfully reprocessed video %s", video.ID.Hex())
	}

	return nil
}

// MigrateVideoFieldNames fixes videos that have data in camelCase fields instead of snake_case
func (s *VideoService) MigrateVideoFieldNames(ctx context.Context) error {
	// Find videos that have hlsPath but empty hls_path
	filter := bson.M{
		"hlsPath": bson.M{"$exists": true, "$ne": ""},
		"$or": []bson.M{
			{"hls_path": ""},
			{"hls_path": bson.M{"$exists": false}},
		},
	}

	cursor, err := s.videoCollection.Find(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to find videos for field migration: %w", err)
	}
	defer cursor.Close(ctx)

	var videos []bson.M
	if err = cursor.All(ctx, &videos); err != nil {
		return fmt.Errorf("failed to decode videos for field migration: %w", err)
	}

	log.Printf("Found %d videos needing field name migration", len(videos))

	for _, video := range videos {
		videoID := video["_id"].(primitive.ObjectID)
		log.Printf("Migrating field names for video %s", videoID.Hex())

		updateFields := bson.M{}
		unsetFields := bson.M{}

		// Migrate hlsPath to hls_path
		if hlsPath, exists := video["hlsPath"]; exists && hlsPath != "" {
			updateFields["hls_path"] = hlsPath
			unsetFields["hlsPath"] = ""
		}

		// Migrate updatedAt to updated_at if needed
		if updatedAt, exists := video["updatedAt"]; exists {
			updateFields["updated_at"] = updatedAt
			unsetFields["updatedAt"] = ""
		}

		// Migrate createdAt to created_at if needed  
		if createdAt, exists := video["createdAt"]; exists {
			updateFields["created_at"] = createdAt
			unsetFields["createdAt"] = ""
		}

		if len(updateFields) > 0 {
			update := bson.M{
				"$set": updateFields,
			}
			if len(unsetFields) > 0 {
				update["$unset"] = unsetFields
			}

			_, err = s.videoCollection.UpdateOne(ctx, bson.M{"_id": videoID}, update)
			if err != nil {
				log.Printf("Failed to migrate field names for video %s: %v", videoID.Hex(), err)
				continue
			}

			log.Printf("Successfully migrated field names for video %s", videoID.Hex())
		}
	}

	return nil
}

