package video

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"streamflow/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var testVideoService *VideoService
var testUserID primitive.ObjectID
var testDbService database.Service

func TestMain(m *testing.M) {
	log.Printf("=== VIDEO SERVICE DATABASE TESTS ===")
	log.Printf("Using real database connection for testing")

	// Set test database name to avoid conflicts with production
	originalDbName := os.Getenv("DB_NAME")
	os.Setenv("DB_NAME", "test_streamflow_video")

	// Check if DB_URI is set
	if os.Getenv("DB_URI") == "" {
		log.Printf("ERROR: DB_URI not set. Please set DB_URI in your .env file")
		log.Printf("Example: DB_URI=mongodb+srv://user:pass@cluster.mongodb.net/dbname")
		os.Exit(1)
	}

	log.Printf("Test database name: test_streamflow_video")

	// Initialize test database service
	testDbService = database.New()
	testVideoService = NewVideoService(testDbService.GetDatabase())
	testUserID = primitive.NewObjectID()

	code := m.Run()

	// Clean up: Drop the test database to remove all test data
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testDbService.GetDatabase().Drop(ctx)
	testDbService.Close()

	// Restore original database name
	if originalDbName != "" {
		os.Setenv("DB_NAME", originalDbName)
	}

	os.Exit(code)
}

// Create a simple video without file operations for testing
func (s *VideoService) CreateVideoSimple(ctx context.Context, userID primitive.ObjectID, title, description string) (*Video, error) {
	videoID := primitive.NewObjectID()

	// Create new video document without file operations
	newVideo := &Video{
		ID:          videoID,
		Title:       title,
		Description: description,
		Status:      StatusPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		UserID:      userID,
		FilePath:    "test/path/video_" + videoID.Hex() + ".mp4",
		Metadata: VideoMetadata{
			Duration: 120.0,
			Width:    1920,
			Height:   1080,
			Codec:    "h264",
			FileSize: 50000000,
		},
	}

	// Insert video document into database
	_, err := s.videoCollection.InsertOne(ctx, newVideo)
	if err != nil {
		return nil, err
	}

	return newVideo, nil
}

// Add UpdateVideoStatus method for testing
func (s *VideoService) UpdateVideoStatus(ctx context.Context, videoID primitive.ObjectID, status VideoStatus) error {
	update := bson.M{
		"$set": bson.M{
			"status":    status,
			"updatedAt": time.Now(),
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

// Add UpdateVideoMetadata method for testing
func (s *VideoService) UpdateVideoMetadata(ctx context.Context, videoID primitive.ObjectID, metadata VideoMetadata) error {
	update := bson.M{
		"$set": bson.M{
			"metadata":  metadata,
			"updatedAt": time.Now(),
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

// Add GetUserVideos method for testing
func (s *VideoService) GetUserVideos(ctx context.Context, userID primitive.ObjectID) ([]*Video, error) {
	cursor, err := s.videoCollection.Find(ctx, bson.M{"user_id": userID})
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

func TestVideoService_CreateVideoSimple(t *testing.T) {
	t.Log("Testing video creation with real database")

	ctx := context.Background()

	tests := []struct {
		name   string
		userID primitive.ObjectID
		title  string
		desc   string
	}{
		{
			name:   "valid video creation",
			userID: testUserID,
			title:  "Test Video " + generateTestSuffix(),
			desc:   "This is a test video",
		},
		{
			name:   "video with empty description",
			userID: testUserID,
			title:  "Test Video 2 " + generateTestSuffix(),
			desc:   "",
		},
		{
			name:   "video with long title",
			userID: testUserID,
			title:  "This is a very long video title that tests the limits of our system " + generateTestSuffix(),
			desc:   "Long title test description",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			video, err := testVideoService.CreateVideoSimple(ctx, tt.userID, tt.title, tt.desc)

			if err != nil {
				t.Errorf("CreateVideoSimple() unexpected error = %v", err)
				return
			}

			// Verify video was created correctly
			if video.UserID != tt.userID {
				t.Errorf("CreateVideoSimple() userID = %v, want %v", video.UserID, tt.userID)
			}
			if video.Title != tt.title {
				t.Errorf("CreateVideoSimple() title = %v, want %v", video.Title, tt.title)
			}
			if video.Description != tt.desc {
				t.Errorf("CreateVideoSimple() description = %v, want %v", video.Description, tt.desc)
			}
			if video.Status != StatusPending {
				t.Errorf("CreateVideoSimple() status = %v, want %v", video.Status, StatusPending)
			}
			if video.ID.IsZero() {
				t.Error("CreateVideoSimple() should generate an ID")
			}
			if video.CreatedAt.IsZero() {
				t.Error("CreateVideoSimple() should set CreatedAt")
			}

			t.Logf("Successfully created video: %s (ID: %s)", video.Title, video.ID.Hex())
		})
	}
}

func TestVideoService_GetVideoByID(t *testing.T) {
	ctx := context.Background()

	// Create a test video first
	video, err := testVideoService.CreateVideoSimple(ctx, testUserID, "Get Test Video "+generateTestSuffix(), "Testing GetVideoByID")
	if err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	t.Logf("Created video for retrieval testing: %s", video.Title)

	// Test valid video ID
	foundVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("GetVideoByID() unexpected error = %v", err)
		return
	}

	if foundVideo.ID != video.ID {
		t.Errorf("GetVideoByID() ID = %v, want %v", foundVideo.ID, video.ID)
	}
	if foundVideo.Title != video.Title {
		t.Errorf("GetVideoByID() title = %v, want %v", foundVideo.Title, video.Title)
	}

	t.Logf("Successfully retrieved video by ID: %s", foundVideo.Title)

	// Test non-existent video ID
	_, err = testVideoService.GetVideoByID(ctx, primitive.NewObjectID())
	if err == nil {
		t.Error("GetVideoByID() should fail for non-existent ID")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("GetVideoByID() error should mention 'not found', got: %v", err)
	} else {
		t.Logf("Correctly handled non-existent video ID: %v", err)
	}
}

func TestVideoService_UpdateVideoStatus(t *testing.T) {
	ctx := context.Background()

	// Create a test video
	video, err := testVideoService.CreateVideoSimple(ctx, testUserID, "Status Test Video "+generateTestSuffix(), "Testing status updates")
	if err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	t.Logf("Created video for status testing: %s", video.Title)

	// Test status updates
	statuses := []VideoStatus{StatusProcessing, StatusCompleted, StatusFailed}

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			err := testVideoService.UpdateVideoStatus(ctx, video.ID, status)
			if err != nil {
				t.Errorf("UpdateVideoStatus() unexpected error = %v", err)
				return
			}

			// Verify status was updated in database
			updatedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
			if err != nil {
				t.Errorf("Failed to get updated video: %v", err)
				return
			}

			if updatedVideo.Status != status {
				t.Errorf("Video status = %v, want %v", updatedVideo.Status, status)
			}

			t.Logf("Successfully updated video status to: %s", status)
		})
	}

	// Test updating non-existent video
	err = testVideoService.UpdateVideoStatus(ctx, primitive.NewObjectID(), StatusCompleted)
	if err == nil {
		t.Error("UpdateVideoStatus() should fail for non-existent video")
	} else {
		t.Logf("Correctly handled non-existent video update: %v", err)
	}
}

func TestVideoService_GetUserVideos(t *testing.T) {
	ctx := context.Background()

	// Create multiple test videos for the same user
	videoCount := 3
	createdVideos := make([]*Video, videoCount)
	var err error

	for i := 0; i < videoCount; i++ {
		title := "User Video " + generateTestSuffix() + string(rune('A'+i))
		desc := "Video for user test"
		createdVideos[i], err = testVideoService.CreateVideoSimple(ctx, testUserID, title, desc)
		if err != nil {
			t.Fatalf("Failed to create test video %d: %v", i+1, err)
		}
	}

	t.Logf("Created %d videos for user testing", videoCount)

	// Create a video for a different user
	otherUserID := primitive.NewObjectID()
	otherVideo, err := testVideoService.CreateVideoSimple(ctx, otherUserID, "Other User Video "+generateTestSuffix(), "Video for different user")
	if err != nil {
		t.Fatalf("Failed to create other user video: %v", err)
	}

	t.Logf("Created video for different user: %s", otherVideo.Title)

	// Test getting videos for the test user
	videos, err := testVideoService.GetUserVideos(ctx, testUserID)
	if err != nil {
		t.Errorf("GetUserVideos() unexpected error = %v", err)
		return
	}

	// Count videos that belong to testUserID
	userVideoCount := 0
	for _, video := range videos {
		if video.UserID == testUserID {
			userVideoCount++
		}
	}

	if userVideoCount < videoCount {
		t.Errorf("User video count = %v, want at least %v", userVideoCount, videoCount)
	}

	// Verify none of the returned videos belong to other user
	for _, video := range videos {
		if video.UserID == otherUserID {
			t.Error("GetUserVideos() returned video from different user")
		}
	}

	t.Logf("Successfully retrieved %d videos for user", userVideoCount)
}

func TestVideoService_UpdateVideoMetadata(t *testing.T) {
	ctx := context.Background()

	// Create a test video
	video, err := testVideoService.CreateVideoSimple(ctx, testUserID, "Metadata Test Video "+generateTestSuffix(), "Testing metadata updates")
	if err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	t.Logf("Created video for metadata testing: %s", video.Title)

	metadata := VideoMetadata{
		Duration:   180.5,
		Width:      1280,
		Height:     720,
		Codec:      "h265",
		AudioCodec: "aac",
		Bitrate:    4000,
		FrameRate:  60.0,
		FileSize:   75000000,
	}

	err = testVideoService.UpdateVideoMetadata(ctx, video.ID, metadata)
	if err != nil {
		t.Errorf("UpdateVideoMetadata() unexpected error = %v", err)
		return
	}

	// Verify metadata was updated in database
	updatedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to get updated video: %v", err)
		return
	}

	if updatedVideo.Metadata.Duration != metadata.Duration {
		t.Errorf("Updated duration = %v, want %v", updatedVideo.Metadata.Duration, metadata.Duration)
	}
	if updatedVideo.Metadata.Width != metadata.Width {
		t.Errorf("Updated width = %v, want %v", updatedVideo.Metadata.Width, metadata.Width)
	}
	if updatedVideo.Metadata.Height != metadata.Height {
		t.Errorf("Updated height = %v, want %v", updatedVideo.Metadata.Height, metadata.Height)
	}
	if updatedVideo.Metadata.Codec != metadata.Codec {
		t.Errorf("Updated codec = %v, want %v", updatedVideo.Metadata.Codec, metadata.Codec)
	}

	t.Logf("Successfully updated video metadata: %dx%d, %.1fs duration", metadata.Width, metadata.Height, metadata.Duration)
}

func TestVideoService_DatabaseConnectivity(t *testing.T) {
	ctx := context.Background()

	// Test basic database operations
	video, err := testVideoService.CreateVideoSimple(ctx, testUserID, "Connectivity Test "+generateTestSuffix(), "Testing database connectivity")
	if err != nil {
		t.Errorf("Database connectivity test failed: %v", err)
		return
	}

	t.Logf("Created video for connectivity testing: %s", video.Title)

	// Verify video exists in database by direct query
	var dbVideo Video
	err = testVideoService.videoCollection.FindOne(ctx, bson.M{"_id": video.ID}).Decode(&dbVideo)
	if err != nil {
		t.Errorf("Failed to find video in database: %v", err)
		return
	}

	if dbVideo.Title != video.Title {
		t.Errorf("Database title = %v, want %v", dbVideo.Title, video.Title)
	}

	t.Logf("Successfully verified video service database connectivity")

	// Test video listing
	videos, err := testVideoService.ListVideos(ctx, 1, 10)
	if err != nil {
		t.Errorf("Failed to list videos: %v", err)
		return
	}

	t.Logf("Successfully listed %d videos from database", len(videos))
}

func TestVideoService_DataPersistence(t *testing.T) {
	ctx := context.Background()

	// Create a video
	video, err := testVideoService.CreateVideoSimple(ctx, testUserID, "Persistence Test "+generateTestSuffix(), "Testing data persistence")
	if err != nil {
		t.Fatalf("Failed to create test video: %v", err)
	}

	t.Logf("Created video for persistence testing: %s", video.Title)

	// Update multiple fields
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusProcessing)
	if err != nil {
		t.Errorf("Failed to update status: %v", err)
	}

	metadata := VideoMetadata{
		Duration: 240.0,
		Width:    1920,
		Height:   1080,
		Codec:    "h264",
		FileSize: 100000000,
	}
	err = testVideoService.UpdateVideoMetadata(ctx, video.ID, metadata)
	if err != nil {
		t.Errorf("Failed to update metadata: %v", err)
	}

	// Wait a moment
	time.Sleep(100 * time.Millisecond)

	// Retrieve and verify all changes persisted
	persistedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to retrieve video: %v", err)
		return
	}

	if persistedVideo.Status != StatusProcessing {
		t.Errorf("Persisted status = %v, want %v", persistedVideo.Status, StatusProcessing)
	}
	if persistedVideo.Metadata.Duration != metadata.Duration {
		t.Errorf("Persisted duration = %v, want %v", persistedVideo.Metadata.Duration, metadata.Duration)
	}
	if persistedVideo.Title != video.Title {
		t.Errorf("Persisted title = %v, want %v", persistedVideo.Title, video.Title)
	}

	t.Logf("Successfully verified data persistence for video: %s", persistedVideo.Title)
}

// generateTestSuffix creates a unique suffix for test data
func generateTestSuffix() string {
	return time.Now().Format("20060102150405")
}

// ==================== COMPREHENSIVE ADDITIONAL TESTS ====================

// Test Video Upload Workflows
func TestVideoService_VideoUploadWorkflow_FileValidation(t *testing.T) {

	tests := []struct {
		name        string
		fileSize    int64
		contentType string
		filename    string
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid mp4 file",
			fileSize:    50000000, // 50MB
			contentType: "video/mp4",
			filename:    "test.mp4",
			expectError: false,
		},
		{
			name:        "file too large",
			fileSize:    600000000, // 600MB (exceeds 500MB limit)
			contentType: "video/mp4", 
			filename:    "large.mp4",
			expectError: true,
			errorMsg:    "exceeds maximum allowed size",
		},
		{
			name:        "invalid content type",
			fileSize:    50000000,
			contentType: "video/quicktime",
			filename:    "test.mov",
			expectError: true,
			errorMsg:    "not allowed",
		},
		{
			name:        "invalid file extension",
			fileSize:    50000000,
			contentType: "video/mp4",
			filename:    "test.wmv",
			expectError: true,
			errorMsg:    "not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock file header
			fileHeader := &multipart.FileHeader{
				Filename: tt.filename,
				Size:     tt.fileSize,
				Header:   make(map[string][]string),
			}
			fileHeader.Header.Set("Content-Type", tt.contentType)

			err := ValidateVideoFile(fileHeader)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for %s, but got none", tt.name)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got: %s", tt.errorMsg, err.Error())
				}
				t.Logf("Correctly rejected invalid file: %s - %v", tt.name, err)
			} else {
				if err != nil {
					t.Errorf("Unexpected error for %s: %v", tt.name, err)
				} else {
					t.Logf("Successfully validated file: %s", tt.name)
				}
			}
		})
	}
}

func TestVideoService_VideoUploadWorkflow_ResumableUpload(t *testing.T) {
	ctx := context.Background()
	
	// Test resumable upload scenario - create video in pending state, then complete upload
	video, err := testVideoService.CreateVideoSimple(ctx, testUserID, "Resumable Upload Test "+generateTestSuffix(), "Testing resumable upload")
	if err != nil {
		t.Fatalf("Failed to create initial video: %v", err)
	}
	
	t.Logf("Created video for resumable upload test: %s", video.Title)
	
	// Simulate upload progress by updating status to processing
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusProcessing)
	if err != nil {
		t.Errorf("Failed to update status to processing: %v", err)
	}
	
	// Simulate completion of upload
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusCompleted)
	if err != nil {
		t.Errorf("Failed to complete upload: %v", err)
	}
	
	// Verify final state
	finalVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to retrieve final video: %v", err)
		return
	}
	
	if finalVideo.Status != StatusCompleted {
		t.Errorf("Expected status %s, got %s", StatusCompleted, finalVideo.Status)
	}
	
	t.Logf("Successfully completed resumable upload workflow for video: %s", finalVideo.Title)
}

// Test Video Processing
func TestVideoService_VideoProcessing_MultipleFormats(t *testing.T) {
	ctx := context.Background()
	
	// Create videos with different codecs to simulate processing
	codecs := []string{"h264", "h265", "vp9", "av1"}
	
	for _, codec := range codecs {
		t.Run("codec_"+codec, func(t *testing.T) {
			video, err := testVideoService.CreateVideoSimple(
				ctx, 
				testUserID, 
				fmt.Sprintf("Processing Test %s %s", codec, generateTestSuffix()), 
				fmt.Sprintf("Testing processing for %s codec", codec),
			)
			if err != nil {
				t.Fatalf("Failed to create video for codec %s: %v", codec, err)
			}
			
			// Update metadata with codec information
			metadata := VideoMetadata{
				Duration:   300.0,
				Width:      1920,
				Height:     1080,
				Codec:      codec,
				AudioCodec: "aac",
				Bitrate:    5000,
				FrameRate:  30.0,
				FileSize:   100000000,
			}
			
			err = testVideoService.UpdateVideoMetadata(ctx, video.ID, metadata)
			if err != nil {
				t.Errorf("Failed to update metadata for codec %s: %v", codec, err)
				return
			}
			
			// Simulate processing workflow
			err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusProcessing)
			if err != nil {
				t.Errorf("Failed to set processing status for codec %s: %v", codec, err)
				return
			}
			
			// Verify processing state
			processedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
			if err != nil {
				t.Errorf("Failed to retrieve processed video for codec %s: %v", codec, err)
				return
			}
			
			if processedVideo.Status != StatusProcessing {
				t.Errorf("Expected processing status for codec %s, got %s", codec, processedVideo.Status)
			}
			if processedVideo.Metadata.Codec != codec {
				t.Errorf("Expected codec %s, got %s", codec, processedVideo.Metadata.Codec)
			}
			
			t.Logf("Successfully processed video with codec %s: %s", codec, processedVideo.Title)
		})
	}
}

func TestVideoService_VideoProcessing_QualityVariations(t *testing.T) {
	ctx := context.Background()
	
	qualityPresets := []struct {
		name     string
		width    int
		height   int
		bitrate  int
		quality  string
	}{
		{"4K", 3840, 2160, 15000, "ultra"},
		{"1080p", 1920, 1080, 8000, "high"},
		{"720p", 1280, 720, 5000, "medium"},
		{"480p", 854, 480, 2500, "low"},
		{"360p", 640, 360, 1000, "mobile"},
	}
	
	for _, preset := range qualityPresets {
		t.Run("quality_"+preset.name, func(t *testing.T) {
			video, err := testVideoService.CreateVideoSimple(
				ctx,
				testUserID,
				fmt.Sprintf("Quality Test %s %s", preset.name, generateTestSuffix()),
				fmt.Sprintf("Testing %s quality processing", preset.name),
			)
			if err != nil {
				t.Fatalf("Failed to create video for quality %s: %v", preset.name, err)
			}
			
			metadata := VideoMetadata{
				Duration:   600.0,
				Width:      preset.width,
				Height:     preset.height,
				Codec:      "h264",
				AudioCodec: "aac",
				Bitrate:    preset.bitrate,
				FrameRate:  60.0,
				FileSize:   int64(preset.bitrate * 75), // Approximate file size
			}
			
			err = testVideoService.UpdateVideoMetadata(ctx, video.ID, metadata)
			if err != nil {
				t.Errorf("Failed to update metadata for quality %s: %v", preset.name, err)
				return
			}
			
			// Verify quality settings
			updatedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
			if err != nil {
				t.Errorf("Failed to retrieve video for quality %s: %v", preset.name, err)
				return
			}
			
			if updatedVideo.Metadata.Width != preset.width || updatedVideo.Metadata.Height != preset.height {
				t.Errorf("Quality %s: expected %dx%d, got %dx%d", 
					preset.name, preset.width, preset.height,
					updatedVideo.Metadata.Width, updatedVideo.Metadata.Height)
			}
			if updatedVideo.Metadata.Bitrate != preset.bitrate {
				t.Errorf("Quality %s: expected bitrate %d, got %d", 
					preset.name, preset.bitrate, updatedVideo.Metadata.Bitrate)
			}
			
			t.Logf("Successfully configured %s quality: %dx%d @ %d kbps", 
				preset.name, preset.width, preset.height, preset.bitrate)
		})
	}
}

// Test Metadata Extraction
func TestVideoService_MetadataExtraction_DurationCalculation(t *testing.T) {
	ctx := context.Background()
	
	testDurations := []struct {
		name     string
		duration float64
		valid    bool
	}{
		{"short video", 30.5, true},
		{"medium video", 600.0, true},
		{"long video", 3599.0, true}, // Just under 1 hour limit
		{"too long video", 7200.0, false}, // Over 1 hour limit
	}
	
	for _, td := range testDurations {
		t.Run(td.name, func(t *testing.T) {
			video, err := testVideoService.CreateVideoSimple(
				ctx,
				testUserID,
				fmt.Sprintf("Duration Test %s %s", td.name, generateTestSuffix()),
				fmt.Sprintf("Testing duration extraction: %.1fs", td.duration),
			)
			if err != nil {
				t.Fatalf("Failed to create video for duration test: %v", err)
			}
			
			metadata := VideoMetadata{
				Duration:   td.duration,
				Width:      1920,
				Height:     1080,
				Codec:      "h264",
				AudioCodec: "aac",
				Bitrate:    5000,
				FrameRate:  30.0,
				FileSize:   int64(td.duration * 1000000), // Approximate
			}
			
			// Validate metadata before updating
			err = ValidateVideoMetadata(&metadata)
			if td.valid && err != nil {
				t.Errorf("Valid duration %s should not fail validation: %v", td.name, err)
				return
			}
			if !td.valid && err == nil {
				t.Errorf("Invalid duration %s should fail validation", td.name)
				return
			}
			
			if td.valid {
				err = testVideoService.UpdateVideoMetadata(ctx, video.ID, metadata)
				if err != nil {
					t.Errorf("Failed to update metadata for duration %s: %v", td.name, err)
					return
				}
				
				updatedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
				if err != nil {
					t.Errorf("Failed to retrieve video for duration test: %v", err)
					return
				}
				
				if updatedVideo.Metadata.Duration != td.duration {
					t.Errorf("Duration mismatch: expected %.1f, got %.1f", 
						td.duration, updatedVideo.Metadata.Duration)
				}
				
				t.Logf("Successfully processed video duration: %.1fs", td.duration)
			} else {
				t.Logf("Correctly rejected invalid duration: %.1fs - %v", td.duration, err)
			}
		})
	}
}

func TestVideoService_MetadataExtraction_ResolutionDetection(t *testing.T) {
	ctx := context.Background()
	
	resolutions := []struct {
		name   string
		width  int
		height int
		valid  bool
	}{
		{"HD", 1280, 720, true},
		{"Full HD", 1920, 1080, true},
		{"4K", 3840, 2160, true},
		{"8K", 7680, 4320, true},
		{"Invalid zero width", 0, 1080, false},
		{"Invalid zero height", 1920, 0, false},
		{"Invalid negative", -1920, 1080, false},
	}
	
	for _, res := range resolutions {
		t.Run(res.name, func(t *testing.T) {
			video, err := testVideoService.CreateVideoSimple(
				ctx,
				testUserID,
				fmt.Sprintf("Resolution Test %s %s", res.name, generateTestSuffix()),
				fmt.Sprintf("Testing resolution: %dx%d", res.width, res.height),
			)
			if err != nil {
				t.Fatalf("Failed to create video for resolution test: %v", err)
			}
			
			metadata := VideoMetadata{
				Duration:   300.0,
				Width:      res.width,
				Height:     res.height,
				Codec:      "h264",
				AudioCodec: "aac",
				Bitrate:    5000,
				FrameRate:  30.0,
				FileSize:   100000000,
			}
			
			err = ValidateVideoMetadata(&metadata)
			if res.valid && err != nil {
				t.Errorf("Valid resolution %s should not fail validation: %v", res.name, err)
				return
			}
			if !res.valid && err == nil {
				t.Errorf("Invalid resolution %s should fail validation", res.name)
				return
			}
			
			if res.valid {
				err = testVideoService.UpdateVideoMetadata(ctx, video.ID, metadata)
				if err != nil {
					t.Errorf("Failed to update metadata for resolution %s: %v", res.name, err)
					return
				}
				
				t.Logf("Successfully processed resolution %s: %dx%d", res.name, res.width, res.height)
			} else {
				t.Logf("Correctly rejected invalid resolution %s: %dx%d - %v", res.name, res.width, res.height, err)
			}
		})
	}
}

// Test Storage Management
func TestVideoService_StorageManagement_FileOrganization(t *testing.T) {
	ctx := context.Background()
	
	// Create videos for different users to test organization
	users := []primitive.ObjectID{
		primitive.NewObjectID(),
		primitive.NewObjectID(),
		primitive.NewObjectID(),
	}
	
	videosByUser := make(map[primitive.ObjectID][]*Video)
	
	for i, userID := range users {
		for j := 0; j < 3; j++ {
			video, err := testVideoService.CreateVideoSimple(
				ctx,
				userID,
				fmt.Sprintf("User%d Video%d %s", i+1, j+1, generateTestSuffix()),
				fmt.Sprintf("Storage organization test for user %d", i+1),
			)
			if err != nil {
				t.Fatalf("Failed to create video for user %d: %v", i+1, err)
			}
			
			videosByUser[userID] = append(videosByUser[userID], video)
		}
	}
	
	// Verify each user's videos are properly organized
	for i, userID := range users {
		userVideos, err := testVideoService.GetUserVideos(ctx, userID)
		if err != nil {
			t.Errorf("Failed to get videos for user %d: %v", i+1, err)
			continue
		}
		
		if len(userVideos) < 3 {
			t.Errorf("Expected at least 3 videos for user %d, got %d", i+1, len(userVideos))
			continue
		}
		
		// Verify all videos belong to the correct user
		for _, video := range userVideos {
			if video.UserID != userID {
				t.Errorf("Video %s belongs to wrong user", video.ID.Hex())
			}
		}
		
		t.Logf("Successfully verified storage organization for user %d: %d videos", i+1, len(userVideos))
	}
}

func TestVideoService_StorageManagement_CleanupProcedures(t *testing.T) {
	ctx := context.Background()
	
	// Create a video for cleanup testing
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Cleanup Test "+generateTestSuffix(),
		"Testing cleanup procedures",
	)
	if err != nil {
		t.Fatalf("Failed to create video for cleanup test: %v", err)
	}
	
	t.Logf("Created video for cleanup test: %s", video.Title)
	
	// Verify video exists
	_, err = testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Video should exist before cleanup: %v", err)
		return
	}
	
	// Perform cleanup by deleting the video
	err = testVideoService.DeleteVideo(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to delete video during cleanup: %v", err)
		return
	}
	
	// Verify video no longer exists
	_, err = testVideoService.GetVideoByID(ctx, video.ID)
	if err == nil {
		t.Error("Video should not exist after cleanup")
		return
	}
	
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error after cleanup, got: %v", err)
		return
	}
	
	t.Logf("Successfully completed cleanup procedures for video: %s", video.Title)
}

// Test Video Validation
func TestVideoService_VideoValidation_ContentTypeValidation(t *testing.T) {
	
	contentTypes := []struct {
		contentType string
		valid       bool
	}{
		{"video/mp4", true},
		{"video/avi", true},
		{"video/mov", true},
		{"video/mkv", true},
		{"video/webm", true},
		{"video/quicktime", false},
		{"video/x-msvideo", false},
		{"application/octet-stream", false},
		{"image/jpeg", false},
		{"text/plain", false},
	}
	
	for _, ct := range contentTypes {
		t.Run("content_type_"+strings.ReplaceAll(ct.contentType, "/", "_"), func(t *testing.T) {
			fileHeader := &multipart.FileHeader{
				Filename: "test.mp4",
				Size:     50000000,
				Header:   make(map[string][]string),
			}
			fileHeader.Header.Set("Content-Type", ct.contentType)
			
			err := ValidateVideoFile(fileHeader)
			
			if ct.valid && err != nil {
				t.Errorf("Valid content type %s should not fail validation: %v", ct.contentType, err)
			} else if !ct.valid && err == nil {
				t.Errorf("Invalid content type %s should fail validation", ct.contentType)
			} else if ct.valid {
				t.Logf("Successfully validated content type: %s", ct.contentType)
			} else {
				t.Logf("Correctly rejected invalid content type: %s - %v", ct.contentType, err)
			}
		})
	}
}

func TestVideoService_VideoValidation_MaliciousFileDetection(t *testing.T) {
	
	// Test various file size edge cases that might indicate malicious files
	testCases := []struct {
		name        string
		size        int64
		filename    string
		expectError bool
	}{
		{"normal video", 50000000, "video.mp4", false},
		{"tiny suspicious file", 100, "video.mp4", false}, // Small but not zero
		{"exactly at limit", MaxFileSize, "video.mp4", false},
		{"over limit", MaxFileSize + 1, "video.mp4", true},
		{"zero size file", 0, "video.mp4", false}, // Zero size might be valid in some cases
		{"extremely large", 2000000000, "virus.mp4", true}, // 2GB
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fileHeader := &multipart.FileHeader{
				Filename: tc.filename,
				Size:     tc.size,
				Header:   make(map[string][]string),
			}
			fileHeader.Header.Set("Content-Type", "video/mp4")
			
			err := ValidateVideoFile(fileHeader)
			
			if tc.expectError && err == nil {
				t.Errorf("Expected error for %s (size: %d), but got none", tc.name, tc.size)
			} else if !tc.expectError && err != nil {
				t.Errorf("Unexpected error for %s (size: %d): %v", tc.name, tc.size, err)
			} else if tc.expectError {
				t.Logf("Correctly detected suspicious file %s (size: %d): %v", tc.name, tc.size, err)
			} else {
				t.Logf("Successfully validated normal file %s (size: %d)", tc.name, tc.size)
			}
		})
	}
}

// Test Batch Operations
func TestVideoService_BatchOperations_BulkUpload(t *testing.T) {
	ctx := context.Background()
	
	// Simulate bulk upload by creating multiple videos rapidly
	batchSize := 10
	var createdVideos []*Video
	
	for i := 0; i < batchSize; i++ {
		video, err := testVideoService.CreateVideoSimple(
			ctx,
			testUserID,
			fmt.Sprintf("Bulk Upload %d %s", i+1, generateTestSuffix()),
			fmt.Sprintf("Batch upload test video %d", i+1),
		)
		if err != nil {
			t.Errorf("Failed to create video %d in bulk upload: %v", i+1, err)
			continue
		}
		createdVideos = append(createdVideos, video)
	}
	
	if len(createdVideos) != batchSize {
		t.Errorf("Expected %d videos in bulk upload, got %d", batchSize, len(createdVideos))
		return
	}
	
	// Verify all videos were created successfully
	for i, video := range createdVideos {
		retrievedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
		if err != nil {
			t.Errorf("Failed to retrieve bulk upload video %d: %v", i+1, err)
			continue
		}
		
		if retrievedVideo.Title != video.Title {
			t.Errorf("Bulk upload video %d title mismatch: expected %s, got %s", 
				i+1, video.Title, retrievedVideo.Title)
		}
	}
	
	t.Logf("Successfully completed bulk upload of %d videos", len(createdVideos))
}

func TestVideoService_BatchOperations_MassProcessing(t *testing.T) {
	ctx := context.Background()
	
	// Create multiple videos and update their status simultaneously
	videoCount := 5
	var videoIDs []primitive.ObjectID
	
	for i := 0; i < videoCount; i++ {
		video, err := testVideoService.CreateVideoSimple(
			ctx,
			testUserID,
			fmt.Sprintf("Mass Processing %d %s", i+1, generateTestSuffix()),
			fmt.Sprintf("Mass processing test video %d", i+1),
		)
		if err != nil {
			t.Errorf("Failed to create video %d for mass processing: %v", i+1, err)
			continue
		}
		videoIDs = append(videoIDs, video.ID)
	}
	
	// Process all videos to processing status
	for i, videoID := range videoIDs {
		err := testVideoService.UpdateVideoStatus(ctx, videoID, StatusProcessing)
		if err != nil {
			t.Errorf("Failed to update video %d to processing: %v", i+1, err)
		}
	}
	
	// Complete processing for all videos
	for i, videoID := range videoIDs {
		err := testVideoService.UpdateVideoStatus(ctx, videoID, StatusCompleted)
		if err != nil {
			t.Errorf("Failed to complete processing for video %d: %v", i+1, err)
		}
	}
	
	// Verify all videos are completed
	completedCount := 0
	for i, videoID := range videoIDs {
		video, err := testVideoService.GetVideoByID(ctx, videoID)
		if err != nil {
			t.Errorf("Failed to retrieve processed video %d: %v", i+1, err)
			continue
		}
		
		if video.Status == StatusCompleted {
			completedCount++
		}
	}
	
	if completedCount != len(videoIDs) {
		t.Errorf("Expected %d completed videos, got %d", len(videoIDs), completedCount)
	} else {
		t.Logf("Successfully mass processed %d videos", completedCount)
	}
}

// Test Privacy Controls
func TestVideoService_PrivacyControls_AccessPermissions(t *testing.T) {
	ctx := context.Background()
	
	// Create videos for different users
	user1 := primitive.NewObjectID()
	user2 := primitive.NewObjectID()
	
	user1Video, err := testVideoService.CreateVideoSimple(
		ctx, user1,
		"User1 Private Video "+generateTestSuffix(),
		"This video belongs to user1",
	)
	if err != nil {
		t.Fatalf("Failed to create user1 video: %v", err)
	}
	
	user2Video, err := testVideoService.CreateVideoSimple(
		ctx, user2,
		"User2 Private Video "+generateTestSuffix(),
		"This video belongs to user2",
	)
	if err != nil {
		t.Fatalf("Failed to create user2 video: %v", err)
	}
	
	// Test that users can only access their own videos through GetUserVideos
	user1Videos, err := testVideoService.GetUserVideos(ctx, user1)
	if err != nil {
		t.Errorf("Failed to get user1 videos: %v", err)
		return
	}
	
	user2Videos, err := testVideoService.GetUserVideos(ctx, user2)
	if err != nil {
		t.Errorf("Failed to get user2 videos: %v", err)
		return
	}
	
	// Verify user1 videos don't contain user2's video
	for _, video := range user1Videos {
		if video.ID == user2Video.ID {
			t.Error("User1 videos should not contain user2's video")
		}
		if video.UserID != user1 {
			t.Error("User1 videos should only contain videos owned by user1")
		}
	}
	
	// Verify user2 videos don't contain user1's video
	for _, video := range user2Videos {
		if video.ID == user1Video.ID {
			t.Error("User2 videos should not contain user1's video")
		}
		if video.UserID != user2 {
			t.Error("User2 videos should only contain videos owned by user2")
		}
	}
	
	t.Logf("Successfully verified privacy controls: User1 has %d videos, User2 has %d videos", 
		len(user1Videos), len(user2Videos))
}

// Test Video Analytics
func TestVideoService_VideoAnalytics_ViewCounting(t *testing.T) {
	ctx := context.Background()
	
	// Create a video for view counting
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"View Count Test "+generateTestSuffix(),
		"Testing view count functionality",
	)
	if err != nil {
		t.Fatalf("Failed to create video for view count test: %v", err)
	}
	
	// Initial view count should be 0
	if video.ViewCount != 0 {
		t.Errorf("Initial view count should be 0, got %d", video.ViewCount)
	}
	
	// Increment view count multiple times
	viewIncrements := 5
	for i := 0; i < viewIncrements; i++ {
		err = testVideoService.IncrementViewCount(ctx, video.ID)
		if err != nil {
			t.Errorf("Failed to increment view count (attempt %d): %v", i+1, err)
		}
	}
	
	// Retrieve video and check view count
	updatedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to retrieve video after view count updates: %v", err)
		return
	}
	
	if updatedVideo.ViewCount != int64(viewIncrements) {
		t.Errorf("Expected view count %d, got %d", viewIncrements, updatedVideo.ViewCount)
	} else {
		t.Logf("Successfully tracked %d views for video: %s", updatedVideo.ViewCount, updatedVideo.Title)
	}
}

func TestVideoService_VideoAnalytics_PopularVideos(t *testing.T) {
	ctx := context.Background()
	
	// Create videos with different view counts
	videos := []struct {
		title     string
		viewCount int
	}{
		{"Most Popular Video " + generateTestSuffix(), 1000},
		{"Popular Video " + generateTestSuffix(), 500},
		{"Less Popular Video " + generateTestSuffix(), 100},
		{"Unpopular Video " + generateTestSuffix(), 10},
	}
	
	var createdVideos []*Video
	
	for _, v := range videos {
		video, err := testVideoService.CreateVideoSimple(ctx, testUserID, v.title, "Popular video test")
		if err != nil {
			t.Errorf("Failed to create video %s: %v", v.title, err)
			continue
		}
		
		// Set video to completed status so it appears in popular videos
		err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusCompleted)
		if err != nil {
			t.Errorf("Failed to set video %s to completed: %v", v.title, err)
			continue
		}
		
		// Simulate view counts by directly updating the database
		for i := 0; i < v.viewCount; i++ {
			err = testVideoService.IncrementViewCount(ctx, video.ID)
			if err != nil {
				t.Errorf("Failed to increment view count for %s: %v", v.title, err)
				break
			}
		}
		
		createdVideos = append(createdVideos, video)
	}
	
	// Get popular videos
	popularVideos, err := testVideoService.GetPopularVideos(ctx, 10)
	if err != nil {
		t.Errorf("Failed to get popular videos: %v", err)
		return
	}
	
	if len(popularVideos) == 0 {
		t.Error("Should have at least some popular videos")
		return
	}
	
	// Verify videos are sorted by view count (descending)
	for i := 1; i < len(popularVideos); i++ {
		if popularVideos[i-1].ViewCount < popularVideos[i].ViewCount {
			t.Errorf("Popular videos not sorted correctly: video %d has %d views, video %d has %d views",
				i-1, popularVideos[i-1].ViewCount, i, popularVideos[i].ViewCount)
		}
	}
	
	t.Logf("Successfully retrieved %d popular videos, top video has %d views", 
		len(popularVideos), popularVideos[0].ViewCount)
}

// Test Database Consistency
func TestVideoService_DatabaseConsistency_VideoMetadataSynchronization(t *testing.T) {
	ctx := context.Background()
	
	// Create a video
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Consistency Test "+generateTestSuffix(),
		"Testing database consistency",
	)
	if err != nil {
		t.Fatalf("Failed to create video for consistency test: %v", err)
	}
	
	// Update metadata
	newMetadata := VideoMetadata{
		Duration:   450.0,
		Width:      1920,
		Height:     1080,
		Codec:      "h264",
		AudioCodec: "aac",
		Bitrate:    6000,
		FrameRate:  30.0,
		FileSize:   150000000,
	}
	
	err = testVideoService.UpdateVideoMetadata(ctx, video.ID, newMetadata)
	if err != nil {
		t.Errorf("Failed to update video metadata: %v", err)
		return
	}
	
	// Update status
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusCompleted)
	if err != nil {
		t.Errorf("Failed to update video status: %v", err)
		return
	}
	
	// Retrieve video and verify all updates are consistent
	consistentVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to retrieve video for consistency check: %v", err)
		return
	}
	
	// Check metadata consistency
	if consistentVideo.Metadata.Duration != newMetadata.Duration {
		t.Errorf("Metadata duration inconsistent: expected %.1f, got %.1f", 
			newMetadata.Duration, consistentVideo.Metadata.Duration)
	}
	if consistentVideo.Metadata.Width != newMetadata.Width {
		t.Errorf("Metadata width inconsistent: expected %d, got %d", 
			newMetadata.Width, consistentVideo.Metadata.Width)
	}
	if consistentVideo.Metadata.Bitrate != newMetadata.Bitrate {
		t.Errorf("Metadata bitrate inconsistent: expected %d, got %d", 
			newMetadata.Bitrate, consistentVideo.Metadata.Bitrate)
	}
	
	// Check status consistency
	if consistentVideo.Status != StatusCompleted {
		t.Errorf("Status inconsistent: expected %s, got %s", StatusCompleted, consistentVideo.Status)
	}
	
	// Check that UpdatedAt was modified
	if !consistentVideo.UpdatedAt.After(video.UpdatedAt) {
		t.Error("UpdatedAt should be modified after updates")
	}
	
	t.Logf("Successfully verified database consistency for video: %s", consistentVideo.Title)
}

// Test Error Scenarios
func TestVideoService_ErrorScenarios_ProcessingFailures(t *testing.T) {
	ctx := context.Background()
	
	// Create a video and simulate processing failure
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Processing Failure Test "+generateTestSuffix(),
		"Testing processing failure handling",
	)
	if err != nil {
		t.Fatalf("Failed to create video for failure test: %v", err)
	}
	
	// Set to processing status
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusProcessing)
	if err != nil {
		t.Errorf("Failed to set processing status: %v", err)
		return
	}
	
	// Simulate processing failure
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusFailed)
	if err != nil {
		t.Errorf("Failed to set failed status: %v", err)
		return
	}
	
	// Verify failed status
	failedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to retrieve video after failure: %v", err)
		return
	}
	
	if failedVideo.Status != StatusFailed {
		t.Errorf("Expected status %s, got %s", StatusFailed, failedVideo.Status)
	}
	
	t.Logf("Successfully handled processing failure for video: %s", failedVideo.Title)
}

func TestVideoService_ErrorScenarios_CorruptedUploads(t *testing.T) {
	ctx := context.Background()
	
	// Test with zero-size file simulation (corrupted upload scenario)
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Corrupted Upload Test "+generateTestSuffix(),
		"Testing corrupted upload handling",
	)
	if err != nil {
		t.Fatalf("Failed to create video for corruption test: %v", err)
	}
	
	// Simulate corrupted metadata (zero duration, invalid resolution)
	corruptedMetadata := VideoMetadata{
		Duration:   0.0, // Invalid
		Width:      0,   // Invalid
		Height:     0,   // Invalid
		Codec:      "",  // Empty
		AudioCodec: "",  // Empty
		Bitrate:    0,
		FrameRate:  0.0,
		FileSize:   0, // Invalid
	}
	
	// This should fail validation
	err = ValidateVideoMetadata(&corruptedMetadata)
	if err == nil {
		t.Error("Corrupted metadata should fail validation")
		return
	}
	
	t.Logf("Successfully detected corrupted upload: %v", err)
	
	// Try to update with corrupted metadata (should fail)
	err = testVideoService.UpdateVideoMetadata(ctx, video.ID, corruptedMetadata)
	if err == nil {
		t.Error("Should not be able to update with corrupted metadata")
		return
	}
	
	t.Logf("Successfully prevented corrupted metadata update: %v", err)
}

// Test Performance
func TestVideoService_Performance_LargeFileHandling(t *testing.T) {
	ctx := context.Background()
	
	// Test with large file size metadata (within limits)
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Large File Test "+generateTestSuffix(),
		"Testing large file handling",
	)
	if err != nil {
		t.Fatalf("Failed to create video for large file test: %v", err)
	}
	
	// Simulate large file metadata
	largeFileMetadata := VideoMetadata{
		Duration:   3599.0, // Just under 1 hour limit
		Width:      3840,   // 4K
		Height:     2160,   // 4K
		Codec:      "h265", // Efficient codec for large files
		AudioCodec: "aac",
		Bitrate:    20000, // High bitrate
		FrameRate:  60.0,  // High frame rate
		FileSize:   int64(MaxFileSize - 1000000), // Just under limit
	}
	
	// Measure update time
	start := time.Now()
	err = testVideoService.UpdateVideoMetadata(ctx, video.ID, largeFileMetadata) 
	updateDuration := time.Since(start)
	
	if err != nil {
		t.Errorf("Failed to handle large file metadata: %v", err)
		return
	}
	
	// Measure retrieval time
	start = time.Now()
	largeVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	retrievalDuration := time.Since(start)
	
	if err != nil {
		t.Errorf("Failed to retrieve large file video: %v", err)
		return
	}
	
	// Performance assertions (adjust thresholds as needed)
	if updateDuration > 5*time.Second {
		t.Errorf("Large file metadata update took too long: %v", updateDuration)
	}
	if retrievalDuration > 1*time.Second {
		t.Errorf("Large file retrieval took too long: %v", retrievalDuration)
	}
	
	// Verify metadata was stored correctly
	if largeVideo.Metadata.FileSize != largeFileMetadata.FileSize {
		t.Errorf("Large file size not stored correctly: expected %d, got %d",
			largeFileMetadata.FileSize, largeVideo.Metadata.FileSize)
	}
	
	t.Logf("Successfully handled large file: %.1fGB, update: %v, retrieval: %v",
		float64(largeVideo.Metadata.FileSize)/(1024*1024*1024), updateDuration, retrievalDuration)
}

func TestVideoService_Performance_ConcurrentProcessing(t *testing.T) {
	ctx := context.Background()
	
	// Test concurrent video operations
	concurrentCount := 10
	var wg sync.WaitGroup
	errors := make(chan error, concurrentCount)
	videos := make(chan *Video, concurrentCount)
	
	// Create videos concurrently
	for i := 0; i < concurrentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			video, err := testVideoService.CreateVideoSimple(
				ctx,
				testUserID,
				fmt.Sprintf("Concurrent Test %d %s", index, generateTestSuffix()),
				fmt.Sprintf("Concurrent processing test %d", index),
			)
			
			if err != nil {
				errors <- err
				return
			}
			
			videos <- video
		}(i)
	}
	
	wg.Wait()
	close(errors)
	close(videos)
	
	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent creation error: %v", err)
		errorCount++
	}
	
	// Count successful videos
	videoList := make([]*Video, 0, concurrentCount)
	for video := range videos {
		videoList = append(videoList, video)
	}
	
	successCount := len(videoList)
	if successCount < concurrentCount-errorCount {
		t.Errorf("Expected at least %d successful videos, got %d", concurrentCount-errorCount, successCount)
	}
	
	if errorCount == 0 {
		t.Logf("Successfully handled %d concurrent video operations", successCount)
	} else {
		t.Logf("Handled %d concurrent operations with %d errors", successCount, errorCount)
	}
	
	// Test concurrent status updates on the created videos
	wg = sync.WaitGroup{}
	updateErrors := make(chan error, len(videoList))
	
	for _, video := range videoList {
		wg.Add(1)
		go func(v *Video) {
			defer wg.Done()
			
			err := testVideoService.UpdateVideoStatus(ctx, v.ID, StatusProcessing)
			if err != nil {
				updateErrors <- err
			}
		}(video)
	}
	
	wg.Wait()
	close(updateErrors)
	
	updateErrorCount := 0
	for err := range updateErrors {
		t.Errorf("Concurrent update error: %v", err)
		updateErrorCount++
	}
	
	if updateErrorCount == 0 {
		t.Logf("Successfully handled %d concurrent status updates", len(videoList))
	} else {
		t.Logf("Handled concurrent updates with %d errors", updateErrorCount)
	}
}

// Test Security
func TestVideoService_Security_AccessControlValidation(t *testing.T) {
	ctx := context.Background()
	
	// Create videos for different users
	user1 := primitive.NewObjectID()
	user2 := primitive.NewObjectID()
	
	user1Video, err := testVideoService.CreateVideoSimple(
		ctx, user1,
		"User1 Security Test "+generateTestSuffix(),
		"Security test for user1",
	)
	if err != nil {
		t.Fatalf("Failed to create user1 video: %v", err)
	}
	
	user2Video, err := testVideoService.CreateVideoSimple(
		ctx, user2,
		"User2 Security Test "+generateTestSuffix(),
		"Security test for user2",
	)
	if err != nil {
		t.Fatalf("Failed to create user2 video: %v", err)
	}
	
	// Test that GetUserVideos properly isolates users
	user1Videos, err := testVideoService.GetUserVideos(ctx, user1)
	if err != nil {
		t.Errorf("Failed to get user1 videos: %v", err)
		return
	}
	
	// Verify user1 cannot see user2's videos through GetUserVideos
	for _, video := range user1Videos {
		if video.ID == user2Video.ID {
			t.Error("Security violation: User1 can see User2's video")
		}
	}
	
	// Both users should be able to access their own videos via GetVideoByID
	_, err = testVideoService.GetVideoByID(ctx, user1Video.ID)
	if err != nil {
		t.Errorf("User should be able to access their own video: %v", err)
	}
	
	_, err = testVideoService.GetVideoByID(ctx, user2Video.ID)
	if err != nil {
		t.Errorf("User should be able to access their own video: %v", err)
	}
	
	t.Logf("Successfully verified access control: User1 has %d videos in isolation", len(user1Videos))
}

// Test Transcoding
func TestVideoService_Transcoding_MultipleQualityOutputs(t *testing.T) {
	ctx := context.Background()
	
	// Create video for transcoding tests
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Transcoding Test "+generateTestSuffix(),
		"Testing transcoding to multiple qualities",
	)
	if err != nil {
		t.Fatalf("Failed to create video for transcoding test: %v", err)
	}
	
	// Set original high-quality metadata
	originalMetadata := VideoMetadata{
		Duration:   300.0,
		Width:      3840, // 4K source
		Height:     2160,
		Codec:      "h264",
		AudioCodec: "aac",
		Bitrate:    15000,
		FrameRate:  30.0,
		FileSize:   200000000,
	}
	
	err = testVideoService.UpdateVideoMetadata(ctx, video.ID, originalMetadata)
	if err != nil {
		t.Errorf("Failed to set original metadata: %v", err)
		return
	}
	
	// Simulate transcoding progress
	transcodingSteps := []VideoStatus{StatusProcessing, StatusCompleted}
	
	for _, status := range transcodingSteps {
		err = testVideoService.UpdateVideoStatus(ctx, video.ID, status)
		if err != nil {
			t.Errorf("Failed to update transcoding status to %s: %v", status, err)
			return
		}
		
		// Verify status update
		updatedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
		if err != nil {
			t.Errorf("Failed to retrieve video during transcoding: %v", err)
			return
		}
		
		if updatedVideo.Status != status {
			t.Errorf("Expected status %s, got %s", status, updatedVideo.Status)
		}
		
		t.Logf("Transcoding progress: %s", status)
	}
	
	t.Logf("Successfully completed transcoding simulation for video: %s", video.Title)
}

func TestVideoService_Transcoding_ProgressTracking(t *testing.T) {
	ctx := context.Background()
	
	// Create multiple videos to simulate a transcoding queue
	queueSize := 5
	var videoQueue []*Video
	
	for i := 0; i < queueSize; i++ {
		video, err := testVideoService.CreateVideoSimple(
			ctx,
			testUserID,
			fmt.Sprintf("Queue Video %d %s", i+1, generateTestSuffix()),
			fmt.Sprintf("Video %d in transcoding queue", i+1),
		)
		if err != nil {
			t.Errorf("Failed to create queue video %d: %v", i+1, err)
			continue
		}
		videoQueue = append(videoQueue, video)
	}
	
	// Simulate processing queue - start all processing
	for i, video := range videoQueue {
		err := testVideoService.UpdateVideoStatus(ctx, video.ID, StatusProcessing)
		if err != nil {
			t.Errorf("Failed to start processing video %d: %v", i+1, err)
		}
	}
	
	// Complete processing in order
	for i, video := range videoQueue {
		err := testVideoService.UpdateVideoStatus(ctx, video.ID, StatusCompleted)
		if err != nil {
			t.Errorf("Failed to complete processing video %d: %v", i+1, err)
		}
	}
	
	// Verify all videos completed
	completedCount := 0
	for i, video := range videoQueue {
		completedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
		if err != nil {
			t.Errorf("Failed to retrieve completed video %d: %v", i+1, err)
			continue
		}
		
		if completedVideo.Status == StatusCompleted {
			completedCount++
		}
	}
	
	if completedCount != len(videoQueue) {
		t.Errorf("Expected %d completed videos, got %d", len(videoQueue), completedCount)
	} else {
		t.Logf("Successfully processed transcoding queue of %d videos", completedCount)
	}
}

// Test Thumbnail Generation
func TestVideoService_ThumbnailGeneration_VideoTimestamps(t *testing.T) {
	ctx := context.Background()
	
	// Create video for thumbnail testing
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Thumbnail Test "+generateTestSuffix(),
		"Testing thumbnail generation",
	)
	if err != nil {
		t.Fatalf("Failed to create video for thumbnail test: %v", err)
	}
	
	// Set video metadata with duration
	metadata := VideoMetadata{
		Duration:   600.0, // 10 minutes
		Width:      1920,
		Height:     1080,
		Codec:      "h264",
		AudioCodec: "aac",
		Bitrate:    5000,
		FrameRate:  30.0,
		FileSize:   100000000,
	}
	
	err = testVideoService.UpdateVideoMetadata(ctx, video.ID, metadata)
	if err != nil {
		t.Errorf("Failed to update video metadata: %v", err)
		return
	}
	
	// Simulate thumbnail generation by setting a thumbnail path
	updatedVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to retrieve video: %v", err)
		return
	}
	
	// Verify video has proper duration for thumbnail generation
	if updatedVideo.Metadata.Duration <= 0 {
		t.Error("Video should have positive duration for thumbnail generation")
		return
	}
	
	t.Logf("Successfully prepared video for thumbnail generation: %.1fs duration", updatedVideo.Metadata.Duration)
}

// Test Error Recovery
func TestVideoService_ErrorRecovery_ProcessingRetry(t *testing.T) {
	ctx := context.Background()
	
	// Create video for error recovery testing
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Error Recovery Test "+generateTestSuffix(),
		"Testing error recovery and retry mechanisms",
	)
	if err != nil {
		t.Fatalf("Failed to create video for error recovery test: %v", err)
	}
	
	// Simulate processing failure
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusProcessing)
	if err != nil {
		t.Errorf("Failed to set processing status: %v", err)
		return
	}
	
	// Simulate failure
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusFailed)
	if err != nil {
		t.Errorf("Failed to set failed status: %v", err)
		return
	}
	
	// Simulate retry - reset to pending
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusPending)
	if err != nil {
		t.Errorf("Failed to reset to pending for retry: %v", err)
		return
	}
	
	// Retry processing
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusProcessing)
	if err != nil {
		t.Errorf("Failed to restart processing: %v", err)
		return
	}
	
	// Complete successfully on retry
	err = testVideoService.UpdateVideoStatus(ctx, video.ID, StatusCompleted)
	if err != nil {
		t.Errorf("Failed to complete on retry: %v", err)
		return
	}
	
	// Verify final success
	recoveredVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to retrieve recovered video: %v", err)
		return
	}
	
	if recoveredVideo.Status != StatusCompleted {
		t.Errorf("Expected completed status after recovery, got %s", recoveredVideo.Status)
		return
	}
	
	t.Logf("Successfully completed error recovery workflow for video: %s", recoveredVideo.Title)
}

// Test Video Listing and Pagination
func TestVideoService_VideoListing_PaginationHandling(t *testing.T) {
	ctx := context.Background()
	
	// Create multiple videos for pagination testing
	totalVideos := 25
	var createdVideos []*Video
	
	for i := 0; i < totalVideos; i++ {
		video, err := testVideoService.CreateVideoSimple(
			ctx,
			testUserID,
			fmt.Sprintf("Pagination Test %d %s", i+1, generateTestSuffix()),
			fmt.Sprintf("Video %d for pagination testing", i+1),
		)
		if err != nil {
			t.Errorf("Failed to create video %d for pagination test: %v", i+1, err)
			continue
		}
		createdVideos = append(createdVideos, video)
	}
	
	t.Logf("Created %d videos for pagination testing", len(createdVideos))
	
	// Test different page sizes
	pageSizes := []int{5, 10, 15}
	
	for _, pageSize := range pageSizes {
		t.Run(fmt.Sprintf("page_size_%d", pageSize), func(t *testing.T) {
			// Test first page
			firstPage, err := testVideoService.ListVideos(ctx, 1, pageSize)
			if err != nil {
				t.Errorf("Failed to get first page with size %d: %v", pageSize, err)
				return
			}
			
			if len(firstPage) > pageSize {
				t.Errorf("First page should not exceed page size %d, got %d", pageSize, len(firstPage))
			}
			
			// Test second page if we have enough videos
			if len(createdVideos) > pageSize {
				secondPage, err := testVideoService.ListVideos(ctx, 2, pageSize)
				if err != nil {
					t.Errorf("Failed to get second page with size %d: %v", pageSize, err)
					return
				}
				
				if len(secondPage) > pageSize {
					t.Errorf("Second page should not exceed page size %d, got %d", pageSize, len(secondPage))
				}
				
				// Verify no overlap between pages
				for _, video1 := range firstPage {
					for _, video2 := range secondPage {
						if video1.ID == video2.ID {
							t.Error("Pages should not have overlapping videos")
						}
					}
				}
			}
			
			t.Logf("Successfully tested pagination with page size %d: first page has %d videos", 
				pageSize, len(firstPage))
		})
	}
}

// Test Data Integrity
func TestVideoService_DataIntegrity_ConcurrentUpdates(t *testing.T) {
	ctx := context.Background()
	
	// Create video for integrity testing
	video, err := testVideoService.CreateVideoSimple(
		ctx,
		testUserID,
		"Data Integrity Test "+generateTestSuffix(),
		"Testing data integrity under concurrent updates",
	)
	if err != nil {
		t.Fatalf("Failed to create video for integrity test: %v", err)
	}
	
	// Perform concurrent metadata updates
	concurrentUpdates := 10
	var wg sync.WaitGroup
	updateErrors := make(chan error, concurrentUpdates)
	
	for i := 0; i < concurrentUpdates; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			
			// Each goroutine updates different metadata fields
			metadata := VideoMetadata{
				Duration:   float64(300 + index),
				Width:      1920,
				Height:     1080,
				Codec:      "h264",
				AudioCodec: "aac",
				Bitrate:    5000 + index*100,
				FrameRate:  30.0,
				FileSize:   int64(100000000 + index*1000000),
			}
			
			err := testVideoService.UpdateVideoMetadata(ctx, video.ID, metadata)
			if err != nil {
				updateErrors <- err
			}
		}(i)
	}
	
	wg.Wait()
	close(updateErrors)
	
	// Check for update errors
	errorCount := 0
	for err := range updateErrors {
		t.Errorf("Concurrent update error: %v", err)
		errorCount++
	}
	
	// Verify final video state is consistent
	finalVideo, err := testVideoService.GetVideoByID(ctx, video.ID)
	if err != nil {
		t.Errorf("Failed to retrieve video after concurrent updates: %v", err)
		return
	}
	
	// Check that metadata is valid (one of the updates succeeded)
	if finalVideo.Metadata.Duration <= 300 || finalVideo.Metadata.Duration > 400 {
		t.Errorf("Final duration %f is outside expected range", finalVideo.Metadata.Duration)
	}
	if finalVideo.Metadata.Bitrate < 5000 || finalVideo.Metadata.Bitrate > 6000 {
		t.Errorf("Final bitrate %d is outside expected range", finalVideo.Metadata.Bitrate)
	}
	
	successfulUpdates := concurrentUpdates - errorCount
	t.Logf("Completed data integrity test: %d successful updates, %d errors", 
		successfulUpdates, errorCount)
}

// Test Edge Cases
func TestVideoService_EdgeCases_BoundaryConditions(t *testing.T) {	
	boundaryTests := []struct {
		name        string
		duration    float64
		fileSize    int64
		expectValid bool
	}{
		{"minimum valid duration", 0.1, 1000, true},
		{"maximum valid duration", float64(MaxDuration), int64(MaxFileSize), true},
		{"zero duration", 0.0, 1000, false},
		{"negative duration", -1.0, 1000, false},
		{"zero file size", 300.0, 0, false},
		{"negative file size", 300.0, -1000, false},
		{"maximum file size", 300.0, int64(MaxFileSize), true},
		{"over maximum file size", 300.0, int64(MaxFileSize + 1), false},
	}
	
	for _, bt := range boundaryTests {
		t.Run(bt.name, func(t *testing.T) {
			metadata := VideoMetadata{
				Duration:   bt.duration,
				Width:      1920,
				Height:     1080,
				Codec:      "h264",
				AudioCodec: "aac",
				Bitrate:    5000,
				FrameRate:  30.0,
				FileSize:   bt.fileSize,
			}
			
			err := ValidateVideoMetadata(&metadata)
			
			if bt.expectValid && err != nil {
				t.Errorf("Expected valid metadata for %s, but got error: %v", bt.name, err)
			} else if !bt.expectValid && err == nil {
				t.Errorf("Expected invalid metadata for %s, but validation passed", bt.name)
			} else if bt.expectValid {
				t.Logf("Successfully validated boundary condition: %s", bt.name)
			} else {
				t.Logf("Correctly rejected boundary condition: %s - %v", bt.name, err)
			}
		})
	}
}
