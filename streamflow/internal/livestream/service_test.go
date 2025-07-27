package livestream

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"streamflow/internal/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var testLivestreamService *LivestreamService
var testUserID primitive.ObjectID
var testDbService database.Service

func TestMain(m *testing.M) {
	log.Printf("=== LIVESTREAM SERVICE DATABASE TESTS ===")
	log.Printf("Using real database connection for testing")

	// Set test database name to avoid conflicts with production
	originalDbName := os.Getenv("DB_NAME")
	os.Setenv("DB_NAME", "test_streamflow_livestream")

	// Check if DB_URI is set
	if os.Getenv("DB_URI") == "" {
		log.Printf("ERROR: DB_URI not set. Please set DB_URI in your .env file")
		log.Printf("Example: DB_URI=mongodb+srv://user:pass@cluster.mongodb.net/dbname")
		os.Exit(1)
	}

	log.Printf("Test database name: test_streamflow_livestream")

	// Initialize test database service
	testDbService = database.New()
	testLivestreamService = NewLiveStreamService(testDbService.GetDatabase())
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

func TestLivestreamService_StartStream(t *testing.T) {
	t.Log("Testing stream creation with real database")

	ctx := context.Background()

	tests := []struct {
		name   string
		userID primitive.ObjectID
		req    StartStreamRequest
	}{
		{
			name:   "valid stream creation",
			userID: testUserID,
			req: StartStreamRequest{
				Title:       "Test Stream " + generateTestSuffix(),
				Description: "This is a test stream",
			},
		},
		{
			name:   "stream with empty description",
			userID: testUserID,
			req: StartStreamRequest{
				Title:       "Test Stream 2 " + generateTestSuffix(),
				Description: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stream, err := testLivestreamService.StartStream(tt.userID, tt.req)

			if err != nil {
				t.Errorf("StartStream() unexpected error = %v", err)
				return
			}

			// Verify stream was created correctly
			if stream.UserID != tt.userID {
				t.Errorf("StartStream() userID = %v, want %v", stream.UserID, tt.userID)
			}
			if stream.Title != tt.req.Title {
				t.Errorf("StartStream() title = %v, want %v", stream.Title, tt.req.Title)
			}
			if stream.Description != tt.req.Description {
				t.Errorf("StartStream() description = %v, want %v", stream.Description, tt.req.Description)
			}
			if stream.Status != StreamStatusLive {
				t.Errorf("StartStream() status = %v, want %v", stream.Status, StreamStatusLive)
			}
			if stream.StreamKey == "" {
				t.Error("StartStream() should generate a stream key")
			}
			if stream.ViewerCount != 0 {
				t.Errorf("StartStream() initial viewer count = %v, want 0", stream.ViewerCount)
			}
			if stream.StartedAt == nil {
				t.Error("StartStream() should set StartedAt")
			}
			if stream.ID.IsZero() {
				t.Error("StartStream() should generate an ID")
			}

			// Verify stream exists in database
			var dbStream Livestream
			err = testLivestreamService.livestreamCollection.FindOne(ctx, bson.M{"_id": stream.ID}).Decode(&dbStream)
			if err != nil {
				t.Errorf("Stream not found in database: %v", err)
			}

			t.Logf("Successfully created stream: %s (ID: %s, Key: %s)", stream.Title, stream.ID.Hex(), stream.StreamKey)
		})
	}
}

func TestLivestreamService_StopStream(t *testing.T) {
	ctx := context.Background()

	// Create a test stream first
	stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
		Title:       "Stream to Stop " + generateTestSuffix(),
		Description: "Test stopping stream",
	})
	if err != nil {
		t.Fatalf("Failed to create test stream: %v", err)
	}

	t.Logf("Created stream for stop testing: %s", stream.Title)

	tests := []struct {
		name     string
		userID   primitive.ObjectID
		streamID primitive.ObjectID
		wantErr  bool
	}{
		{
			name:     "valid stream stop",
			userID:   testUserID,
			streamID: stream.ID,
			wantErr:  false,
		},
		{
			name:     "unauthorized user",
			userID:   primitive.NewObjectID(),
			streamID: stream.ID,
			wantErr:  true,
		},
		{
			name:     "non-existent stream",
			userID:   testUserID,
			streamID: primitive.NewObjectID(),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := testLivestreamService.StopStream(tt.userID, tt.streamID)

			if tt.wantErr {
				if err == nil {
					t.Errorf("StopStream() expected error, got nil")
				} else {
					t.Logf("Correctly handled invalid stop request: %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("StopStream() unexpected error = %v", err)
				return
			}

			// Verify stream status was updated in database
			var dbStream Livestream
			err = testLivestreamService.livestreamCollection.FindOne(ctx, bson.M{"_id": tt.streamID}).Decode(&dbStream)
			if err != nil {
				t.Errorf("Failed to find updated stream: %v", err)
				return
			}

			if dbStream.Status != StreamStatusEnded {
				t.Errorf("Stream status = %v, want %v", dbStream.Status, StreamStatusEnded)
			}
			if dbStream.EndedAt == nil {
				t.Error("EndedAt should be set when stream is stopped")
			}

			t.Logf("Successfully stopped stream: %s", stream.Title)
		})
	}
}

func TestLivestreamService_GetStreamByKey(t *testing.T) {

	// Create a test stream
	stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
		Title:       "Stream by Key Test " + generateTestSuffix(),
		Description: "Testing GetStreamByKey",
	})
	if err != nil {
		t.Fatalf("Failed to create test stream: %v", err)
	}

	t.Logf("Created stream for key testing: %s (Key: %s)", stream.Title, stream.StreamKey)

	// Test valid stream key
	foundStream, err := testLivestreamService.GetStreamByKey(stream.StreamKey)
	if err != nil {
		t.Errorf("GetStreamByKey() unexpected error = %v", err)
		return
	}

	if foundStream.StreamKey != stream.StreamKey {
		t.Errorf("GetStreamByKey() streamKey = %v, want %v", foundStream.StreamKey, stream.StreamKey)
	}
	if foundStream.ID != stream.ID {
		t.Errorf("GetStreamByKey() ID = %v, want %v", foundStream.ID, stream.ID)
	}

	t.Logf("Successfully found stream by key: %s", foundStream.Title)

	// Test invalid stream key
	_, err = testLivestreamService.GetStreamByKey("invalid-key-" + generateTestSuffix())
	if err == nil {
		t.Error("GetStreamByKey() should fail for invalid key")
	} else {
		t.Logf("Correctly handled invalid stream key: %v", err)
	}
}

func TestLivestreamService_ViewerOperations(t *testing.T) {

	// Create a test stream
	stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
		Title:       "Viewer Test Stream " + generateTestSuffix(),
		Description: "Testing viewer operations",
	})
	if err != nil {
		t.Fatalf("Failed to create test stream: %v", err)
	}

	t.Logf("Created stream for viewer testing: %s", stream.Title)

	// Test adding viewer
	t.Run("AddViewer", func(t *testing.T) {
		err := testLivestreamService.AddViewer(stream.ID)
		if err != nil {
			t.Errorf("AddViewer() unexpected error = %v", err)
		}

		// Verify viewer count increased
		count, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("GetViewerCount() unexpected error = %v", err)
		}
		if count != 1 {
			t.Errorf("Viewer count = %v, want 1", count)
		}

		t.Logf("Successfully added viewer, count now: %d", count)
	})

	// Test adding multiple viewers
	t.Run("AddMultipleViewers", func(t *testing.T) {
		for i := 0; i < 3; i++ {
			err := testLivestreamService.AddViewer(stream.ID)
			if err != nil {
				t.Errorf("AddViewer() unexpected error = %v", err)
			}
		}

		count, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("GetViewerCount() unexpected error = %v", err)
		}
		if count != 4 { // 1 from previous test + 3 new
			t.Errorf("Viewer count = %v, want 4", count)
		}

		t.Logf("Successfully added multiple viewers, count now: %d", count)
	})

	// Test removing viewer
	t.Run("RemoveViewer", func(t *testing.T) {
		err := testLivestreamService.RemoveViewer(stream.ID)
		if err != nil {
			t.Errorf("RemoveViewer() unexpected error = %v", err)
		}

		count, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("GetViewerCount() unexpected error = %v", err)
		}
		if count != 3 {
			t.Errorf("Viewer count = %v, want 3", count)
		}

		t.Logf("Successfully removed viewer, count now: %d", count)
	})
}

func TestLivestreamService_ChatOperations(t *testing.T) {

	// Create a test stream
	stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
		Title:       "Chat Test Stream " + generateTestSuffix(),
		Description: "Testing chat operations",
	})
	if err != nil {
		t.Fatalf("Failed to create test stream: %v", err)
	}

	t.Logf("Created stream for chat testing: %s", stream.Title)

	chatUserID := primitive.NewObjectID()

	// Test sending chat message
	t.Run("SendChatMessage", func(t *testing.T) {
		err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, "testuser", "Hello, world!")
		if err != nil {
			t.Errorf("SendChatMessage() unexpected error = %v", err)
		}

		// Send a few more messages
		messages := []string{"How's everyone doing?", "Great stream!", "Thanks for watching!"}
		for _, msg := range messages {
			err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, "testuser", msg)
			if err != nil {
				t.Errorf("SendChatMessage() unexpected error = %v", err)
			}
		}

		t.Logf("Successfully sent %d chat messages", len(messages)+1)
	})

	// Test retrieving chat messages
	t.Run("GetMessages", func(t *testing.T) {
		messages, err := testLivestreamService.GetMessages(stream.ID)
		if err != nil {
			t.Errorf("GetMessages() unexpected error = %v", err)
			return
		}

		if len(messages) < 4 { // Should have at least 4 messages from above
			t.Errorf("Message count = %v, want at least 4", len(messages))
			return
		}

		// Verify message content
		found := false
		for _, msg := range messages {
			if msg.Message == "Hello, world!" && msg.UserName == "testuser" {
				found = true
				if msg.StreamID != stream.ID {
					t.Errorf("Message streamID = %v, want %v", msg.StreamID, stream.ID)
				}
				break
			}
		}

		if !found {
			t.Error("Expected chat message not found")
		} else {
			t.Logf("Successfully retrieved %d chat messages", len(messages))
		}
	})
}

func TestLivestreamService_ListStreams(t *testing.T) {

	// Create multiple test streams
	streamCount := 3
	createdStreams := make([]*Livestream, streamCount)
	var err error

	for i := 0; i < streamCount; i++ {
		createdStreams[i], err = testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       fmt.Sprintf("List Test Stream %d %s", i+1, generateTestSuffix()),
			Description: fmt.Sprintf("Stream %d for list testing", i+1),
		})
		if err != nil {
			t.Fatalf("Failed to create test stream %d: %v", i+1, err)
		}
	}

	t.Logf("Created %d streams for list testing", streamCount)

	// Stop one stream to test filtering
	_, err = testLivestreamService.StopStream(testUserID, createdStreams[2].ID)
	if err != nil {
		t.Fatalf("Failed to stop test stream: %v", err)
	}

	t.Logf("Stopped one stream to test filtering")

	// Test listing live streams
	liveStreams, err := testLivestreamService.ListStreams()
	if err != nil {
		t.Errorf("ListStreams() unexpected error = %v", err)
		return
	}

	// Count live streams (should be 2 out of 3 we created)
	liveCount := 0
	for _, stream := range liveStreams {
		// Only count our test streams
		for _, created := range createdStreams {
			if stream.ID == created.ID && stream.Status == StreamStatusLive {
				liveCount++
			}
		}
	}

	if liveCount != 2 { // 3 created - 1 stopped = 2 live
		t.Errorf("Live stream count = %v, want 2", liveCount)
	} else {
		t.Logf("Successfully listed streams, found %d live streams out of %d total", liveCount, len(liveStreams))
	}
}

func TestLivestreamService_DatabaseConsistency(t *testing.T) {
	ctx := context.Background()

	// Create a stream
	stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
		Title:       "Consistency Test " + generateTestSuffix(),
		Description: "Testing database consistency",
	})
	if err != nil {
		t.Fatalf("Failed to create test stream: %v", err)
	}

	t.Logf("Created stream for consistency testing: %s", stream.Title)

	// Test data persistence across operations
	t.Run("DataPersistence", func(t *testing.T) {
		// Perform multiple operations
		updates := map[string]interface{}{
			"title":       "Updated Title " + generateTestSuffix(),
			"description": "Updated Description",
		}
		err := testLivestreamService.UpdateStream(stream.ID, updates)
		if err != nil {
			t.Errorf("UpdateStream() error: %v", err)
		}

		// Verify updates persisted
		var dbStream Livestream
		err = testLivestreamService.livestreamCollection.FindOne(ctx, bson.M{"_id": stream.ID}).Decode(&dbStream)
		if err != nil {
			t.Errorf("Failed to find updated stream: %v", err)
		}

		if dbStream.Title != updates["title"] {
			t.Errorf("Updated title = %v, want %v", dbStream.Title, updates["title"])
		}
		if dbStream.Description != updates["description"] {
			t.Errorf("Updated description = %v, want %v", dbStream.Description, updates["description"])
		}

		t.Logf("Successfully verified data persistence for stream: %s", dbStream.Title)
	})
}

// generateTestSuffix creates a unique suffix for test data
func generateTestSuffix() string {
	return time.Now().Format("20060102150405")
}
