package video

import (
	"context"
	"log"
	"os"
	"strings"
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
