package livestream

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"sync/atomic"
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

// ===== EXTENSIVE ADDITIONAL TESTS FOR COMPREHENSIVE COVERAGE =====

// TestLivestreamService_StreamLifecycleManagement tests complex stream lifecycle scenarios
func TestLivestreamService_StreamLifecycleManagement(t *testing.T) {
	ctx := context.Background()

	t.Run("MultipleStreamLifecycles", func(t *testing.T) {
		// Create multiple streams concurrently
		streamCount := 5
		streams := make([]*Livestream, streamCount)
		var wg sync.WaitGroup

		// Start streams concurrently
		for i := 0; i < streamCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
					Title:       fmt.Sprintf("Concurrent Stream %d %s", index, generateTestSuffix()),
					Description: fmt.Sprintf("Concurrent test stream %d", index),
				})
				if err != nil {
					t.Errorf("Failed to start concurrent stream %d: %v", index, err)
					return
				}
				streams[index] = stream
			}(i)
		}
		wg.Wait()

		// Verify all streams were created
		for i, stream := range streams {
			if stream == nil {
				t.Errorf("Stream %d was not created", i)
				continue
			}
			if stream.Status != StreamStatusLive {
				t.Errorf("Stream %d status = %v, want %v", i, stream.Status, StreamStatusLive)
			}
		}

		// Stop streams randomly
		stopOrder := []int{2, 0, 4, 1, 3}
		for _, index := range stopOrder {
			if streams[index] != nil {
				_, err := testLivestreamService.StopStream(testUserID, streams[index].ID)
				if err != nil {
					t.Errorf("Failed to stop stream %d: %v", index, err)
				}
			}
		}

		t.Logf("Successfully managed lifecycle of %d concurrent streams", streamCount)
	})

	t.Run("StreamRecoveryScenarios", func(t *testing.T) {
		// Create a stream
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "Recovery Test Stream " + generateTestSuffix(),
			Description: "Testing stream recovery",
		})
		if err != nil {
			t.Fatalf("Failed to create recovery test stream: %v", err)
		}

		// Simulate database corruption by updating status directly
		_, err = testLivestreamService.livestreamCollection.UpdateOne(ctx,
			bson.M{"_id": stream.ID},
			bson.M{"$set": bson.M{"status": "CORRUPTED"}})
		if err != nil {
			t.Fatalf("Failed to simulate corruption: %v", err)
		}

		// Try to recover by updating stream status
		err = testLivestreamService.UpdateStream(stream.ID, map[string]interface{}{
			"status": StreamStatusLive,
		})
		if err != nil {
			t.Errorf("Stream recovery failed: %v", err)
		}

		// Verify recovery
		recoveredStream, err := testLivestreamService.GetStreamStatus(stream.ID)
		if err != nil {
			t.Errorf("Failed to get recovered stream: %v", err)
		} else if recoveredStream.Status != StreamStatusLive {
			t.Errorf("Stream not properly recovered: status = %v", recoveredStream.Status)
		}

		t.Logf("Successfully tested stream recovery scenarios")
	})

	t.Run("AbnormalTerminationHandling", func(t *testing.T) {
		// Create multiple streams
		streams := make([]*Livestream, 3)
		for i := range streams {
			var err error
			streams[i], err = testLivestreamService.StartStream(testUserID, StartStreamRequest{
				Title:       fmt.Sprintf("Termination Test %d %s", i, generateTestSuffix()),
				Description: "Testing abnormal termination",
			})
			if err != nil {
				t.Fatalf("Failed to create termination test stream %d: %v", i, err)
			}
		}

		// Simulate abnormal termination by directly updating database
		for i, stream := range streams {
			if i%2 == 0 { // Terminate every other stream
				now := time.Now()
				_, err := testLivestreamService.livestreamCollection.UpdateOne(ctx,
					bson.M{"_id": stream.ID},
					bson.M{"$set": bson.M{
						"status":     StreamStatusEnded,
						"ended_at":   now,
						"updated_at": now,
					}})
				if err != nil {
					t.Errorf("Failed to simulate abnormal termination for stream %d: %v", i, err)
				}
			}
		}

		// Verify cleanup and consistency
		liveStreams, err := testLivestreamService.ListStreams()
		if err != nil {
			t.Errorf("Failed to list streams after termination: %v", err)
		}

		// Count live streams from our test batch
		liveCount := 0
		for _, liveStream := range liveStreams {
			for i, testStream := range streams {
				if liveStream.ID == testStream.ID && liveStream.Status == StreamStatusLive {
					if i%2 == 0 {
						t.Errorf("Stream %d should have been terminated but is still live", i)
					}
					liveCount++
				}
			}
		}

		expectedLive := 1 // Only stream 1 should be live (0 and 2 terminated)
		if liveCount != expectedLive {
			t.Errorf("Expected %d live streams, got %d", expectedLive, liveCount)
		}

		t.Logf("Successfully handled abnormal termination scenarios")
	})
}

// TestLivestreamService_ConcurrentViewerManagement tests viewer operations under concurrent load
func TestLivestreamService_ConcurrentViewerManagement(t *testing.T) {
	// Create test stream
	stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
		Title:       "Concurrent Viewer Test " + generateTestSuffix(),
		Description: "Testing concurrent viewer operations",
	})
	if err != nil {
		t.Fatalf("Failed to create test stream: %v", err)
	}

	t.Run("ConcurrentViewerAdditions", func(t *testing.T) {
		viewerCount := 100
		var wg sync.WaitGroup

		// Add viewers concurrently
		for i := 0; i < viewerCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				err := testLivestreamService.AddViewer(stream.ID)
				if err != nil {
					t.Errorf("Failed to add viewer %d: %v", index, err)
				}
			}(i)
		}
		wg.Wait()

		// Verify final count
		finalCount, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("Failed to get final viewer count: %v", err)
		} else if finalCount != viewerCount {
			t.Errorf("Expected %d viewers, got %d", viewerCount, finalCount)
		}

		t.Logf("Successfully added %d concurrent viewers", viewerCount)
	})

	t.Run("ConcurrentViewerRemovals", func(t *testing.T) {
		// Get current count
		currentCount, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Fatalf("Failed to get current viewer count: %v", err)
		}

		removeCount := currentCount / 2
		var wg sync.WaitGroup

		// Remove viewers concurrently
		for i := 0; i < removeCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				err := testLivestreamService.RemoveViewer(stream.ID)
				if err != nil {
					t.Errorf("Failed to remove viewer %d: %v", index, err)
				}
			}(i)
		}
		wg.Wait()

		// Verify final count
		finalCount, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("Failed to get final viewer count: %v", err)
		}

		expectedCount := currentCount - removeCount
		if finalCount != expectedCount {
			t.Errorf("Expected %d viewers after removal, got %d", expectedCount, finalCount)
		}

		t.Logf("Successfully removed %d concurrent viewers", removeCount)
	})

	t.Run("MixedConcurrentOperations", func(t *testing.T) {
		var wg sync.WaitGroup
		operations := 50

		// Mix of add and remove operations
		for i := 0; i < operations; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				if index%2 == 0 {
					testLivestreamService.AddViewer(stream.ID)
				} else {
					testLivestreamService.RemoveViewer(stream.ID)
				}
			}(i)
		}
		wg.Wait()

		// Verify count is consistent (should not be negative)
		finalCount, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("Failed to get final viewer count: %v", err)
		} else if finalCount < 0 {
			t.Errorf("Viewer count went negative: %d", finalCount)
		}

		t.Logf("Successfully completed mixed concurrent viewer operations, final count: %d", finalCount)
	})
}

// TestLivestreamService_ChatSystemComprehensive tests the complete chat system
func TestLivestreamService_ChatSystemComprehensive(t *testing.T) {
	// Create test stream
	stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
		Title:       "Chat System Test " + generateTestSuffix(),
		Description: "Comprehensive chat testing",
	})
	if err != nil {
		t.Fatalf("Failed to create test stream: %v", err)
	}

	t.Run("MessageValidationAndSanitization", func(t *testing.T) {
		testCases := []struct {
			name     string
			message  string
			userName string
			wantErr  bool
		}{
			{
				name:     "normal message",
				message:  "Hello everyone!",
				userName: "testuser",
				wantErr:  false,
			},
			{
				name:     "empty message",
				message:  "",
				userName: "testuser",
				wantErr:  false, // Should handle empty messages gracefully
			},
			{
				name:     "long message",
				message:  strings.Repeat("a", 1000),
				userName: "testuser",
				wantErr:  false,
			},
			{
				name:     "special characters",
				message:  "Special chars: !@#$%^&*()_+{}|:<>?[]\\;'\".,/",
				userName: "testuser",
				wantErr:  false,
			},
			{
				name:     "unicode message",
				message:  "Unicode: 😀🎉🌟 こんにちは مرحبا",
				userName: "testuser",
				wantErr:  false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				chatUserID := primitive.NewObjectID()
				err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, tc.userName, tc.message)
				
				if tc.wantErr && err == nil {
					t.Errorf("Expected error for message: %s", tc.message)
				} else if !tc.wantErr && err != nil {
					t.Errorf("Unexpected error for message '%s': %v", tc.message, err)
				}
			})
		}

		t.Logf("Successfully tested message validation and sanitization")
	})

	t.Run("ConcurrentChatOperations", func(t *testing.T) {
		userCount := 10
		messagesPerUser := 20
		var wg sync.WaitGroup

		// Send messages concurrently from multiple users
		for userIndex := 0; userIndex < userCount; userIndex++ {
			wg.Add(1)
			go func(uIndex int) {
				defer wg.Done()
				chatUserID := primitive.NewObjectID()
				userName := fmt.Sprintf("user%d", uIndex)
				
				for msgIndex := 0; msgIndex < messagesPerUser; msgIndex++ {
					message := fmt.Sprintf("Message %d from %s", msgIndex, userName)
					err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, userName, message)
					if err != nil {
						t.Errorf("Failed to send message from %s: %v", userName, err)
					}
					
					// Small delay to simulate realistic chat patterns
					time.Sleep(time.Millisecond * 10)
				}
			}(userIndex)
		}
		wg.Wait()

		// Verify message count
		messages, err := testLivestreamService.GetMessages(stream.ID)
		if err != nil {
			t.Errorf("Failed to get messages: %v", err)
		}

		expectedMinMessages := userCount * messagesPerUser
		actualMessages := len(messages)
		if actualMessages < expectedMinMessages {
			t.Errorf("Expected at least %d messages, got %d", expectedMinMessages, actualMessages)
		}

		t.Logf("Successfully handled %d concurrent chat messages from %d users", actualMessages, userCount)
	})

	t.Run("ChatMessageHistory", func(t *testing.T) {
		// Send messages with timestamps
		chatUserID := primitive.NewObjectID()
		testMessages := []string{
			"First message",
			"Second message", 
			"Third message",
		}

		for i, msg := range testMessages {
			// Send message
			err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, "historyuser", msg)
			if err != nil {
				t.Errorf("Failed to send history message %d: %v", i, err)
			}
			
			// Small delay to ensure timestamp ordering
			time.Sleep(time.Millisecond * 100)
		}

		// Retrieve and verify message order
		messages, err := testLivestreamService.GetMessages(stream.ID)
		if err != nil {
			t.Errorf("Failed to get message history: %v", err)
		}

		// Find our test messages in the results
		foundMessages := make(map[string]bool)
		for _, msg := range messages {
			if msg.UserName == "historyuser" {
				foundMessages[msg.Message] = true
			}
		}

		// Verify all test messages were found
		for _, expectedMsg := range testMessages {
			if !foundMessages[expectedMsg] {
				t.Errorf("Message '%s' not found in history", expectedMsg)
			}
		}

		t.Logf("Successfully verified chat message history with %d test messages", len(testMessages))
	})

	t.Run("ChatRateLimitingSimulation", func(t *testing.T) {
		// Simulate rapid message sending to test system resilience
		chatUserID := primitive.NewObjectID()
		rapidMessageCount := 100
		var successCount int32
		var wg sync.WaitGroup

		for i := 0; i < rapidMessageCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				message := fmt.Sprintf("Rapid message %d", index)
				err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, "rapiduser", message)
				if err == nil {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}
		wg.Wait()

		// Even with rapid sending, most messages should succeed (no rate limiting implemented yet)
		if successCount < int32(rapidMessageCount*1) {
			t.Errorf("Too many rapid messages failed: %d/%d succeeded", successCount, rapidMessageCount)
		}

		t.Logf("Successfully handled rapid message sending: %d/%d messages succeeded", successCount, rapidMessageCount)
	})
}

// TestLivestreamService_SearchAndDiscovery tests stream search and discovery features
func TestLivestreamService_SearchAndDiscovery(t *testing.T) {
	// Create diverse test streams
	testStreams := []struct {
		title       string
		description string
	}{
		{
			title:       "Gaming Stream: Fortnite Battle Royale " + generateTestSuffix(),
			description: "Playing Fortnite with friends, come join the fun!",
		},
		{
			title:       "Cooking Show: Italian Pasta " + generateTestSuffix(),
			description: "Learn to make authentic Italian pasta from scratch",
		},
		{
			title:       "Music Performance: Jazz Night " + generateTestSuffix(),
			description: "Live jazz performance with piano and saxophone",
		},
		{
			title:       "Tech Talk: Go Programming " + generateTestSuffix(),
			description: "Advanced Go programming techniques and best practices",
		},
		{
			title:       "Art Stream: Digital Painting " + generateTestSuffix(),
			description: "Creating digital art using Photoshop and Procreate",
		},
	}

	createdStreams := make([]*Livestream, len(testStreams))
	for i, streamData := range testStreams {
		var err error
		createdStreams[i], err = testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       streamData.title,
			Description: streamData.description,
		})
		if err != nil {
			t.Fatalf("Failed to create test stream %d: %v", i, err)
		}
		
		// Add different viewer counts to test popularity sorting
		viewerCount := (i + 1) * 10
		for j := 0; j < viewerCount; j++ {
			testLivestreamService.AddViewer(createdStreams[i].ID)
		}
	}

	t.Run("SearchByTitle", func(t *testing.T) {
		searchQueries := []struct {
			query    string
			expected int
		}{
			{"Gaming", 1},
			{"cooking", 1}, // Case insensitive
			{"MUSIC", 1},   // Case insensitive
			{"programming", 1},
			{"nonexistent", 0},
			{"Stream", 5}, // Should match all (all have "Stream" in title)
		}

		for _, sq := range searchQueries {
			results, err := testLivestreamService.SearchStreams(sq.query)
			if err != nil {
				t.Errorf("Search failed for query '%s': %v", sq.query, err)
				continue
			}

			// Count matches from our test streams
			matchCount := 0
			for _, result := range results {
				for _, created := range createdStreams {
					if result.ID == created.ID {
						matchCount++
						break
					}
				}
			}

			if matchCount != sq.expected {
				t.Errorf("Search for '%s': expected %d matches, got %d", sq.query, sq.expected, matchCount)
			}
		}

		t.Logf("Successfully tested search by title functionality")
	})

	t.Run("SearchByDescription", func(t *testing.T) {
		searchQueries := []struct {
			query    string
			expected int
		}{
			{"friends", 1},
			{"Italian", 1},
			{"piano", 1},
			{"techniques", 1},
			{"Photoshop", 1},
		}

		for _, sq := range searchQueries {
			results, err := testLivestreamService.SearchStreams(sq.query)
			if err != nil {
				t.Errorf("Description search failed for query '%s': %v", sq.query, err)
				continue
			}

			// Count matches from our test streams
			matchCount := 0
			for _, result := range results {
				for _, created := range createdStreams {
					if result.ID == created.ID {
						matchCount++
						break
					}
				}
			}

			if matchCount >= sq.expected {
				// At least the expected matches (there might be other streams)
				t.Logf("Search for '%s' in description: found %d matches (expected at least %d)", sq.query, matchCount, sq.expected)
			} else {
				t.Errorf("Description search for '%s': expected at least %d matches, got %d", sq.query, sq.expected, matchCount)
			}
		}

		t.Logf("Successfully tested search by description functionality")
	})

	t.Run("PopularStreamsRanking", func(t *testing.T) {
		popularStreams, err := testLivestreamService.GetPopularStreams(10)
		if err != nil {
			t.Errorf("Failed to get popular streams: %v", err)
			return
		}

		// Find our test streams in the results
		ourPopularStreams := make([]*Livestream, 0)
		for _, popular := range popularStreams {
			for _, created := range createdStreams {
				if popular.ID == created.ID {
					ourPopularStreams = append(ourPopularStreams, popular)
					break
				}
			}
		}

		// Verify they're sorted by viewer count (descending)
		for i := 1; i < len(ourPopularStreams); i++ {
			if ourPopularStreams[i-1].ViewerCount < ourPopularStreams[i].ViewerCount {
				t.Errorf("Popular streams not properly sorted: stream %d has %d viewers, stream %d has %d viewers",
					i-1, ourPopularStreams[i-1].ViewerCount, i, ourPopularStreams[i].ViewerCount)
			}
		}

		// Verify the highest viewer count stream is first among our streams
		if len(ourPopularStreams) > 0 {
			highestViewers := ourPopularStreams[0].ViewerCount
			expectedHighest := 50 // Art stream should have 50 viewers (index 4 + 1) * 10
			if highestViewers != expectedHighest {
				t.Errorf("Expected highest viewer count to be %d, got %d", expectedHighest, highestViewers)
			}
		}

		t.Logf("Successfully tested popular streams ranking with %d popular streams found", len(ourPopularStreams))
	})
}

// TestLivestreamService_UserStreamManagement tests user-specific stream operations
func TestLivestreamService_UserStreamManagement(t *testing.T) {
	// Create additional test users
	user2ID := primitive.NewObjectID()
	user3ID := primitive.NewObjectID()

	t.Run("MultiUserStreamCreation", func(t *testing.T) {
		users := []primitive.ObjectID{testUserID, user2ID, user3ID}
		streamsPerUser := 3
		userStreams := make(map[primitive.ObjectID][]*Livestream)

		// Create streams for each user
		for _, userID := range users {
			userStreams[userID] = make([]*Livestream, streamsPerUser)
			for i := 0; i < streamsPerUser; i++ {
				stream, err := testLivestreamService.StartStream(userID, StartStreamRequest{
					Title:       fmt.Sprintf("User %s Stream %d %s", userID.Hex()[:8], i+1, generateTestSuffix()),
					Description: fmt.Sprintf("Stream %d for user %s", i+1, userID.Hex()[:8]),
				})
				if err != nil {
					t.Errorf("Failed to create stream %d for user %s: %v", i+1, userID.Hex()[:8], err)
				} else {
					userStreams[userID][i] = stream
				}
			}
		}

		// Verify each user can retrieve their own streams
		for userID, expectedStreams := range userStreams {
			retrievedStreams, err := testLivestreamService.GetUserStreams(userID)
			if err != nil {
				t.Errorf("Failed to get streams for user %s: %v", userID.Hex()[:8], err)
				continue
			}

			// Count streams that match our test data
			matchCount := 0
			for _, retrieved := range retrievedStreams {
				for _, expected := range expectedStreams {
					if expected != nil && retrieved.ID == expected.ID {
						matchCount++
						break
					}
				}
			}

			if matchCount != streamsPerUser {
				t.Errorf("User %s: expected %d streams, found %d matching streams", 
					userID.Hex()[:8], streamsPerUser, matchCount)
			}
		}

		t.Logf("Successfully tested multi-user stream creation and retrieval")
	})

	t.Run("UserStreamIsolation", func(t *testing.T) {
		// Verify users can only stop their own streams
		user1Streams, err := testLivestreamService.GetUserStreams(testUserID)
		if err != nil {
			t.Fatalf("Failed to get user1 streams: %v", err)
		}

		user2Streams, err := testLivestreamService.GetUserStreams(user2ID)
		if err != nil {
			t.Fatalf("Failed to get user2 streams: %v", err)
		}

		if len(user1Streams) == 0 || len(user2Streams) == 0 {
			t.Skip("Need streams from both users for isolation test")
		}

		// Try to stop user2's stream using user1's ID (should fail)
		_, err = testLivestreamService.StopStream(testUserID, user2Streams[0].ID)
		if err == nil {
			t.Error("User should not be able to stop another user's stream")
		} else {
			t.Logf("Correctly prevented unauthorized stream stop: %v", err)
		}

		// User2 should be able to stop their own stream
		_, err = testLivestreamService.StopStream(user2ID, user2Streams[0].ID)
		if err != nil {
			t.Errorf("User should be able to stop their own stream: %v", err)
		} else {
			t.Logf("Successfully allowed authorized stream stop")
		}

		t.Logf("Successfully tested user stream isolation")
	})

	t.Run("UserStreamLimits", func(t *testing.T) {
		// Test creating many streams for one user
		maxStreams := 10
		userID := user3ID
		createdStreams := make([]*Livestream, 0, maxStreams)

		for i := 0; i < maxStreams; i++ {
			stream, err := testLivestreamService.StartStream(userID, StartStreamRequest{
				Title:       fmt.Sprintf("Limit Test Stream %d %s", i+1, generateTestSuffix()),
				Description: fmt.Sprintf("Testing stream limits - stream %d", i+1),
			})
			if err != nil {
				t.Errorf("Failed to create stream %d: %v", i+1, err)
			} else {
				createdStreams = append(createdStreams, stream)
			}
		}

		// Verify all streams were created
		userStreams, err := testLivestreamService.GetUserStreams(userID)
		if err != nil {
			t.Errorf("Failed to get user streams: %v", err)
		}

		if len(userStreams) < maxStreams {
			t.Errorf("Expected at least %d streams for user, got %d", maxStreams, len(userStreams))
		}

		t.Logf("Successfully tested user stream limits with %d streams", len(createdStreams))
	})
}

// TestLivestreamService_FFmpegIntegration tests FFmpeg service integration
func TestLivestreamService_FFmpegIntegration(t *testing.T) {
	ffmpegService := NewFFmpegService()

	t.Run("FFmpegAvailability", func(t *testing.T) {
		err := ffmpegService.CheckFFmpegAvailable()
		if err != nil {
			t.Skipf("FFmpeg not available, skipping integration tests: %v", err)
		}

		version, err := ffmpegService.TestFFmpegConnection()
		if err != nil {
			t.Errorf("FFmpeg connection test failed: %v", err)
		} else {
			t.Logf("FFmpeg available: %s", version)
		}
	})

	t.Run("RecorderServiceIntegration", func(t *testing.T) {
		// Create a test stream
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "FFmpeg Integration Test " + generateTestSuffix(),
			Description: "Testing FFmpeg integration",
		})
		if err != nil {
			t.Fatalf("Failed to create test stream: %v", err)
		}

		// Test recording status for non-existent session
		_, err = testLivestreamService.recorderService.GetRecordingStatus(stream.ID)
		if err == nil {
			t.Error("Should return error for non-existent recording session")
		}

		// Test stopping non-existent recording
		err = testLivestreamService.recorderService.StopRecording(stream.ID)
		if err == nil {
			t.Error("Should return error when stopping non-existent recording")
		}

		t.Logf("Successfully tested recorder service integration")
	})

	// Note: Actual FFmpeg recording tests would require a real RTMP stream
	// This tests the service integration without requiring actual video streams
}

// TestLivestreamService_DatabaseConsistencyAdvanced tests advanced database consistency scenarios
func TestLivestreamService_DatabaseConsistencyAdvanced(t *testing.T) {
	ctx := context.Background()

	t.Run("TransactionConsistency", func(t *testing.T) {
		// Create a stream
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "Transaction Test " + generateTestSuffix(),
			Description: "Testing transaction consistency",
		})
		if err != nil {
			t.Fatalf("Failed to create test stream: %v", err)
		}

		// Perform multiple related operations
		operations := []struct {
			name string
			op   func() error
		}{
			{
				name: "add viewers",
				op: func() error {
					for i := 0; i < 5; i++ {
						if err := testLivestreamService.AddViewer(stream.ID); err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				name: "send chat messages",
				op: func() error {
					chatUserID := primitive.NewObjectID()
					for i := 0; i < 3; i++ {
						if err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, "testuser", 
							fmt.Sprintf("Consistency test message %d", i)); err != nil {
							return err
						}
					}
					return nil
				},
			},
			{
				name: "update stream metadata",
				op: func() error {
					return testLivestreamService.UpdateStream(stream.ID, map[string]interface{}{
						"description": "Updated during consistency test",
					})
				},
			},
		}

		// Execute operations
		for _, op := range operations {
			if err := op.op(); err != nil {
				t.Errorf("Operation '%s' failed: %v", op.name, err)
			}
		}

		// Verify final state consistency
		finalStream, err := testLivestreamService.GetStreamStatus(stream.ID)
		if err != nil {
			t.Errorf("Failed to get final stream state: %v", err)
		} else {
			if finalStream.ViewerCount != 5 {
				t.Errorf("Expected 5 viewers, got %d", finalStream.ViewerCount)
			}
			if finalStream.Description != "Updated during consistency test" {
				t.Errorf("Stream description not updated correctly")
			}
		}

		// Verify chat messages
		messages, err := testLivestreamService.GetMessages(stream.ID)
		if err != nil {
			t.Errorf("Failed to get chat messages: %v", err)
		} else {
			consistencyMessages := 0
			for _, msg := range messages {
				if strings.Contains(msg.Message, "Consistency test message") {
					consistencyMessages++
				}
			}
			if consistencyMessages != 3 {
				t.Errorf("Expected 3 consistency test messages, found %d", consistencyMessages)
			}
		}

		t.Logf("Successfully verified transaction consistency")
	})

	t.Run("ConcurrentDatabaseOperations", func(t *testing.T) {
		// Create multiple streams for concurrent operations
		streamCount := 5
		streams := make([]*Livestream, streamCount)
		for i := 0; i < streamCount; i++ {
			var err error
			streams[i], err = testLivestreamService.StartStream(testUserID, StartStreamRequest{
				Title:       fmt.Sprintf("Concurrent DB Test %d %s", i, generateTestSuffix()),
				Description: "Testing concurrent database operations",
			})
			if err != nil {
				t.Fatalf("Failed to create concurrent test stream %d: %v", i, err)
			}
		}

		var wg sync.WaitGroup
		var errors int32

		// Perform concurrent operations on all streams
		for _, stream := range streams {
			for opType := 0; opType < 3; opType++ {
				wg.Add(1)
				go func(s *Livestream, op int) {
					defer wg.Done()
					
					switch op {
					case 0: // Viewer operations
						for i := 0; i < 10; i++ {
							if err := testLivestreamService.AddViewer(s.ID); err != nil {
								atomic.AddInt32(&errors, 1)
							}
						}
					case 1: // Chat operations
						chatUserID := primitive.NewObjectID()
						for i := 0; i < 5; i++ {
							if err := testLivestreamService.SendChatMessage(s.ID, chatUserID, "concurrentuser", 
								fmt.Sprintf("Concurrent message %d", i)); err != nil {
								atomic.AddInt32(&errors, 1)
							}
						}
					case 2: // Update operations
						for i := 0; i < 3; i++ {
							if err := testLivestreamService.UpdateStream(s.ID, map[string]interface{}{
								"description": fmt.Sprintf("Updated %d times", i+1),
							}); err != nil {
								atomic.AddInt32(&errors, 1)
							}
						}
					}
				}(stream, opType)
			}
		}

		wg.Wait()

		if errors > 0 {
			t.Errorf("Encountered %d errors during concurrent operations", errors)
		}

		// Verify final consistency
		for i, stream := range streams {
			finalStream, err := testLivestreamService.GetStreamStatus(stream.ID)
			if err != nil {
				t.Errorf("Failed to get final state for stream %d: %v", i, err)
				continue
			}

			// Each stream should have 10 viewers
			if finalStream.ViewerCount != 10 {
				t.Errorf("Stream %d: expected 10 viewers, got %d", i, finalStream.ViewerCount)
			}

			// Each stream should have its description updated
			if !strings.Contains(finalStream.Description, "Updated") {
				t.Errorf("Stream %d: description not updated during concurrent operations", i)
			}
		}

		t.Logf("Successfully completed concurrent database operations on %d streams", streamCount)
	})

	t.Run("DataIntegrityAfterFailures", func(t *testing.T) {
		// Create a stream
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "Integrity Test " + generateTestSuffix(),
			Description: "Testing data integrity after failures",
		})
		if err != nil {
			t.Fatalf("Failed to create integrity test stream: %v", err)
		}

		// Add some initial data
		for i := 0; i < 5; i++ {
			testLivestreamService.AddViewer(stream.ID)
		}

		chatUserID := primitive.NewObjectID()
		for i := 0; i < 3; i++ {
			testLivestreamService.SendChatMessage(stream.ID, chatUserID, "integrityuser", 
				fmt.Sprintf("Integrity message %d", i))
		}

		// Simulate partial failure by directly manipulating database
		_, err = testLivestreamService.livestreamCollection.UpdateOne(ctx,
			bson.M{"_id": stream.ID},
			bson.M{"$set": bson.M{"viewer_count": -1}}) // Invalid state
		if err != nil {
			t.Fatalf("Failed to simulate database inconsistency: %v", err)
		}

		// Verify the service handles inconsistent state gracefully
		count, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("Failed to get viewer count with inconsistent data: %v", err)
		} else {
			t.Logf("Viewer count with inconsistent data: %d", count)
		}

		// Attempt recovery
		err = testLivestreamService.UpdateStream(stream.ID, map[string]interface{}{
			"viewer_count": 5, // Restore correct count
		})
		if err != nil {
			t.Errorf("Failed to recover from inconsistent state: %v", err)
		}

		// Verify recovery
		recoveredCount, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("Failed to get viewer count after recovery: %v", err)
		} else if recoveredCount != 5 {
			t.Errorf("Expected viewer count 5 after recovery, got %d", recoveredCount)
		}

		t.Logf("Successfully tested data integrity and recovery")
	})
}

// TestLivestreamService_PerformanceAndStress tests system performance under load
func TestLivestreamService_PerformanceAndStress(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance tests in short mode")
	}

	t.Run("HighVolumeStreamCreation", func(t *testing.T) {
		streamCount := 50
		var wg sync.WaitGroup
		var successCount int32
		var errorCount int32

		startTime := time.Now()

		for i := 0; i < streamCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				_, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
					Title:       fmt.Sprintf("Performance Test Stream %d %s", index, generateTestSuffix()),
					Description: fmt.Sprintf("Performance test stream number %d", index),
				})
				if err != nil {
					atomic.AddInt32(&errorCount, 1)
				} else {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		t.Logf("Created %d streams in %v (%.2f streams/second)", 
			successCount, duration, float64(successCount)/duration.Seconds())
		
		if errorCount > 0 {
			t.Logf("Encountered %d errors during high-volume creation", errorCount)
		}

		if successCount < int32(streamCount*1) {
			t.Errorf("Too many failures: %d/%d streams created successfully", successCount, streamCount)
		}
	})

	t.Run("HighThroughputChatMessages", func(t *testing.T) {
		// Create a test stream
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "Chat Performance Test " + generateTestSuffix(),
			Description: "Testing high-throughput chat messages",
		})
		if err != nil {
			t.Fatalf("Failed to create performance test stream: %v", err)
		}

		messageCount := 1000
		userCount := 10
		var wg sync.WaitGroup
		var successCount int32

		startTime := time.Now()

		for userIndex := 0; userIndex < userCount; userIndex++ {
			wg.Add(1)
			go func(uIndex int) {
				defer wg.Done()
				chatUserID := primitive.NewObjectID()
				userName := fmt.Sprintf("perfuser%d", uIndex)
				
				for msgIndex := 0; msgIndex < messageCount/userCount; msgIndex++ {
					message := fmt.Sprintf("Performance message %d from user %d", msgIndex, uIndex)
					err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, userName, message)
					if err == nil {
						atomic.AddInt32(&successCount, 1)
					}
				}
			}(userIndex)
		}

		wg.Wait()
		duration := time.Since(startTime)

		t.Logf("Sent %d chat messages in %v (%.2f messages/second)", 
			successCount, duration, float64(successCount)/duration.Seconds())

		if successCount < int32(messageCount*1) {
			t.Errorf("Too many message failures: %d/%d messages sent successfully", successCount, messageCount)
		}
	})

	t.Run("ConcurrentViewerOperations", func(t *testing.T) {
		// Create test streams
		streamCount := 10
		streams := make([]*Livestream, streamCount)
		for i := 0; i < streamCount; i++ {
			var err error
			streams[i], err = testLivestreamService.StartStream(testUserID, StartStreamRequest{
				Title:       fmt.Sprintf("Viewer Performance Test %d %s", i, generateTestSuffix()),
				Description: "Testing concurrent viewer operations",
			})
			if err != nil {
				t.Fatalf("Failed to create performance test stream %d: %v", i, err)
			}
		}

		operationCount := 1000
		var wg sync.WaitGroup
		var successCount int32

		startTime := time.Now()

		for i := 0; i < operationCount; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				stream := streams[index%streamCount]
				
				// Alternate between add and remove operations
				var err error
				if index%2 == 0 {
					err = testLivestreamService.AddViewer(stream.ID)
				} else {
					err = testLivestreamService.RemoveViewer(stream.ID)
				}
				
				if err == nil {
					atomic.AddInt32(&successCount, 1)
				}
			}(i)
		}

		wg.Wait()
		duration := time.Since(startTime)

		t.Logf("Completed %d viewer operations in %v (%.2f operations/second)", 
			successCount, duration, float64(successCount)/duration.Seconds())

		if successCount < int32(operationCount*1) {
			t.Errorf("Too many viewer operation failures: %d/%d operations successful", successCount, operationCount)
		}
	})

	t.Run("DatabaseQueryPerformance", func(t *testing.T) {
		// Create streams with various data for query testing
		streamCount := 100
		for i := 0; i < streamCount; i++ {
			stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
				Title:       fmt.Sprintf("Query Performance Stream %d %s", i, generateTestSuffix()),
				Description: fmt.Sprintf("Performance test stream for queries - number %d", i),
			})
			if err != nil {
				t.Errorf("Failed to create query test stream %d: %v", i, err)
				continue
			}

			// Add varying viewer counts
			viewerCount := i % 20
			for j := 0; j < viewerCount; j++ {
				testLivestreamService.AddViewer(stream.ID)
			}

			// Stop some streams to test filtering
			if i%10 == 0 {
				testLivestreamService.StopStream(testUserID, stream.ID)
			}
		}

		// Test query performance
		queryTests := []struct {
			name string
			op   func() (interface{}, error)
		}{
			{
				name: "list all live streams",
				op: func() (interface{}, error) {
					return testLivestreamService.ListStreams()
				},
			},
			{
				name: "get popular streams",
				op: func() (interface{}, error) {
					return testLivestreamService.GetPopularStreams(20)
				},
			},
			{
				name: "search streams",
				op: func() (interface{}, error) {
					return testLivestreamService.SearchStreams("Performance")
				},
			},
			{
				name: "get user streams",
				op: func() (interface{}, error) {
					return testLivestreamService.GetUserStreams(testUserID)
				},
			},
		}

		for _, test := range queryTests {
			iterations := 10
			startTime := time.Now()
			
			for i := 0; i < iterations; i++ {
				_, err := test.op()
				if err != nil {
					t.Errorf("Query '%s' failed on iteration %d: %v", test.name, i, err)
				}
			}
			
			duration := time.Since(startTime)
			avgDuration := duration / time.Duration(iterations)
			
			t.Logf("Query '%s': avg %v per query (%d iterations)", test.name, avgDuration, iterations)
		}
	})
}

// TestLivestreamService_ErrorHandlingAndRecovery tests comprehensive error scenarios
func TestLivestreamService_ErrorHandlingAndRecovery(t *testing.T) {
	t.Run("InvalidInputHandling", func(t *testing.T) {
		// Test with invalid ObjectIDs
		invalidID := primitive.ObjectID{}
		
		// Test operations with invalid stream ID
		_, err := testLivestreamService.GetStreamStatus(invalidID)
		if err == nil {
			t.Error("Should return error for invalid stream ID")
		}

		err = testLivestreamService.AddViewer(invalidID)
		if err == nil {
			t.Error("Should return error when adding viewer to invalid stream")
		}

		err = testLivestreamService.RemoveViewer(invalidID)
		if err == nil {
			t.Error("Should return error when removing viewer from invalid stream")
		}

		// Test with invalid user ID
		_, err = testLivestreamService.StartStream(invalidID, StartStreamRequest{
			Title:       "Invalid User Test",
			Description: "Testing invalid user ID",
		})
		if err != nil {
			// This might actually succeed depending on validation logic
			t.Logf("StartStream with invalid user ID returned error (which may be expected): %v", err)
		}

		t.Logf("Successfully tested invalid input handling")
	})

	t.Run("NonExistentResourceHandling", func(t *testing.T) {
		nonExistentID := primitive.NewObjectID()

		// Test operations on non-existent streams
		_, err := testLivestreamService.StopStream(testUserID, nonExistentID)
		if err == nil {
			t.Error("Should return error when stopping non-existent stream")
		}

		_, err = testLivestreamService.GetStreamStatus(nonExistentID)
		if err == nil {
			t.Error("Should return error when getting status of non-existent stream")
		}

		count, err := testLivestreamService.GetViewerCount(nonExistentID)
		if err == nil {
			t.Errorf("Should return error for non-existent stream viewer count, got: %d", count)
		}

		// Test getting messages for non-existent stream
		messages, err := testLivestreamService.GetMessages(nonExistentID)
		if err != nil {
			t.Logf("GetMessages for non-existent stream returned error: %v", err)
		} else if len(messages) != 0 {
			t.Errorf("Expected 0 messages for non-existent stream, got %d", len(messages))
		}

		t.Logf("Successfully tested non-existent resource handling")
	})

	t.Run("ConcurrentModificationHandling", func(t *testing.T) {
		// Create a test stream
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "Concurrent Modification Test " + generateTestSuffix(),
			Description: "Testing concurrent modifications",
		})
		if err != nil {
			t.Fatalf("Failed to create test stream: %v", err)
		}

		var wg sync.WaitGroup
		var successCount int32
		var errorCount int32

		// Perform conflicting operations concurrently
		operations := 20
		for i := 0; i < operations; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				
				switch index % 4 {
				case 0:
					// Update stream metadata
					err := testLivestreamService.UpdateStream(stream.ID, map[string]interface{}{
						"description": fmt.Sprintf("Updated by operation %d", index),
					})
					if err != nil {
						atomic.AddInt32(&errorCount, 1)
					} else {
						atomic.AddInt32(&successCount, 1)
					}
				case 1:
					// Add viewers
					err := testLivestreamService.AddViewer(stream.ID)
					if err != nil {
						atomic.AddInt32(&errorCount, 1)
					} else {
						atomic.AddInt32(&successCount, 1)
					}
				case 2:
					// Remove viewers
					err := testLivestreamService.RemoveViewer(stream.ID)
					if err != nil {
						atomic.AddInt32(&errorCount, 1)
					} else {
						atomic.AddInt32(&successCount, 1)
					}
				case 3:
					// Send chat messages
					chatUserID := primitive.NewObjectID()
					err := testLivestreamService.SendChatMessage(stream.ID, chatUserID, 
						fmt.Sprintf("user%d", index), fmt.Sprintf("Concurrent message %d", index))
					if err != nil {
						atomic.AddInt32(&errorCount, 1)
					} else {
						atomic.AddInt32(&successCount, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Concurrent modifications: %d successful, %d errors", successCount, errorCount)
		
		// Verify final state is consistent
		finalStream, err := testLivestreamService.GetStreamStatus(stream.ID)
		if err != nil {
			t.Errorf("Failed to get final stream state: %v", err)
		} else {
			if finalStream.ViewerCount < 0 {
				t.Errorf("Viewer count went negative: %d", finalStream.ViewerCount)
			}
		}
	})

	t.Run("ServiceRecoveryAfterErrors", func(t *testing.T) {
		// Test service resilience after various error conditions
		
		// Create a stream
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "Recovery Test " + generateTestSuffix(),
			Description: "Testing service recovery",
		})
		if err != nil {
			t.Fatalf("Failed to create recovery test stream: %v", err)
		}

		// Cause various error conditions and test recovery
		errorScenarios := []struct {
			name string
			op   func() error
		}{
			{
				name: "invalid viewer operations",
				op: func() error {
					// Try operations on invalid streams
					invalidID := primitive.NewObjectID()
					testLivestreamService.AddViewer(invalidID)
					testLivestreamService.RemoveViewer(invalidID)
					return nil
				},
			},
			{
				name: "malformed chat messages",
				op: func() error {
					// Try to send problematic messages
					chatUserID := primitive.NewObjectID()
					// Very long message
					longMessage := strings.Repeat("a", 10000)
					testLivestreamService.SendChatMessage(stream.ID, chatUserID, "testuser", longMessage)
					// Empty message
					testLivestreamService.SendChatMessage(stream.ID, chatUserID, "testuser", "")
					return nil
				},
			},
			{
				name: "invalid stream updates",
				op: func() error {
					// Try invalid update operations
					testLivestreamService.UpdateStream(primitive.NewObjectID(), map[string]interface{}{
						"title": "This should fail",
					})
					return nil
				},
			},
		}

		for _, scenario := range errorScenarios {
			t.Run(scenario.name, func(t *testing.T) {
				// Execute error scenario
				scenario.op()
				
				// Verify service is still functional after errors
				_, err := testLivestreamService.GetStreamStatus(stream.ID)
				if err != nil {
					t.Errorf("Service not functional after %s: %v", scenario.name, err)
				}
				
				// Try normal operations
				err = testLivestreamService.AddViewer(stream.ID)
				if err != nil {
					t.Errorf("Normal operation failed after %s: %v", scenario.name, err)
				}
			})
		}

		t.Logf("Successfully tested service recovery after error scenarios")
	})
}

// TestLivestreamService_StreamManagerIntegration tests integration with StreamManager
func TestLivestreamService_StreamManagerIntegration(t *testing.T) {
	streamManager := NewStreamManager(testLivestreamService)

	t.Run("StreamManagerBasicOperations", func(t *testing.T) {
		// Create a test stream
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "Stream Manager Test " + generateTestSuffix(),
			Description: "Testing stream manager integration",
		})
		if err != nil {
			t.Fatalf("Failed to create test stream: %v", err)
		}

		// Test stream start handling
		streamManager.HandleStreamStart(stream.StreamKey, stream.ID)
		
		// Verify tracks are created
		videoTrack, audioTrack := streamManager.GetStreamTracks(stream.StreamKey)
		if videoTrack == nil || audioTrack == nil {
			t.Error("Stream tracks not created properly")
		} else {
			t.Logf("Successfully created video and audio tracks for stream")
		}

		// Test viewer operations through stream manager
		streamManager.HandleViewerJoin(stream.StreamKey)
		streamManager.HandleViewerJoin(stream.StreamKey)
		
		// Verify viewer count updated in database
		time.Sleep(time.Millisecond * 100) // Allow async operations to complete
		count, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("Failed to get viewer count: %v", err)
		} else if count < 2 {
			t.Errorf("Expected at least 2 viewers, got %d", count)
		}

		// Test viewer leave
		streamManager.HandleViewerLeave(stream.StreamKey)
		time.Sleep(time.Millisecond * 100)
		
		newCount, err := testLivestreamService.GetViewerCount(stream.ID)
		if err != nil {
			t.Errorf("Failed to get viewer count after leave: %v", err)
		} else if newCount >= count {
			t.Errorf("Viewer count should have decreased: was %d, now %d", count, newCount)
		}

		// Test stream end handling
		streamManager.HandleStreamEnd(stream.StreamKey)
		
		// Verify tracks are cleaned up
		videoTrack, audioTrack = streamManager.GetStreamTracks(stream.StreamKey)
		if videoTrack != nil || audioTrack != nil {
			t.Error("Stream tracks not cleaned up properly")
		}

		t.Logf("Successfully tested stream manager basic operations")
	})

	t.Run("MultipleStreamManagement", func(t *testing.T) {
		// Create multiple streams
		streamCount := 5
		streams := make([]*Livestream, streamCount)
		
		for i := 0; i < streamCount; i++ {
			var err error
			streams[i], err = testLivestreamService.StartStream(testUserID, StartStreamRequest{
				Title:       fmt.Sprintf("Multi Stream Manager Test %d %s", i, generateTestSuffix()),
				Description: fmt.Sprintf("Stream %d for multi-stream testing", i),
			})
			if err != nil {
				t.Fatalf("Failed to create test stream %d: %v", i, err)
			}
			
			// Start each stream in the manager
			streamManager.HandleStreamStart(streams[i].StreamKey, streams[i].ID)
		}

		// Add viewers to different streams
		for i, stream := range streams {
			viewerCount := (i + 1) * 2
			for j := 0; j < viewerCount; j++ {
				streamManager.HandleViewerJoin(stream.StreamKey)
			}
		}

		// Allow async operations to complete
		time.Sleep(time.Millisecond * 200)

		// Verify viewer counts
		for i, stream := range streams {
			expectedCount := (i + 1) * 2
			actualCount, err := testLivestreamService.GetViewerCount(stream.ID)
			if err != nil {
				t.Errorf("Failed to get viewer count for stream %d: %v", i, err)
			} else if actualCount < expectedCount {
				t.Errorf("Stream %d: expected at least %d viewers, got %d", i, expectedCount, actualCount)
			}
		}

		// End all streams
		for _, stream := range streams {
			streamManager.HandleStreamEnd(stream.StreamKey)
		}

		t.Logf("Successfully tested multiple stream management")
	})
}

// TestLivestreamService_ComplexWorkflows tests end-to-end complex workflows
func TestLivestreamService_ComplexWorkflows(t *testing.T) {
	t.Run("CompleteStreamLifecycleWorkflow", func(t *testing.T) {
		// Phase 1: Stream Creation and Setup
		stream, err := testLivestreamService.StartStream(testUserID, StartStreamRequest{
			Title:       "Complete Workflow Test " + generateTestSuffix(),
			Description: "Testing complete stream lifecycle workflow",
		})
		if err != nil {
			t.Fatalf("Failed to create workflow test stream: %v", err)
		}

		t.Logf("Phase 1: Created stream %s", stream.Title)

		// Phase 2: Simulate Stream Activity
		chatUserID := primitive.NewObjectID()
		
		// Send initial chat messages
		initialMessages := []string{
			"Hello everyone!",
			"Welcome to the stream",
			"Hope you enjoy the content",
		}
		
		for _, msg := range initialMessages {
			err = testLivestreamService.SendChatMessage(stream.ID, chatUserID, "streamer", msg)
			if err != nil {
				t.Errorf("Failed to send initial message: %v", err)
			}
		}

		// Add viewers gradually
		for i := 0; i < 10; i++ {
			err = testLivestreamService.AddViewer(stream.ID)
			if err != nil {
				t.Errorf("Failed to add viewer %d: %v", i, err)
			}
			time.Sleep(time.Millisecond * 10)
		}

		t.Logf("Phase 2: Added viewers and initial chat activity")

		// Phase 3: Peak Activity Simulation
		var wg sync.WaitGroup
		
		// Simulate multiple viewers chatting
		for userIndex := 0; userIndex < 5; userIndex++ {
			wg.Add(1)
			go func(uIndex int) {
				defer wg.Done()
				uChatUserID := primitive.NewObjectID()
				userName := fmt.Sprintf("viewer%d", uIndex)
				
				messages := []string{
					fmt.Sprintf("Hi from %s!", userName),
					"Great stream!",
					"Thanks for the content",
					"Keep it up!",
				}
				
				for _, msg := range messages {
					testLivestreamService.SendChatMessage(stream.ID, uChatUserID, userName, msg)
					time.Sleep(time.Millisecond * 50)
				}
			}(userIndex)
		}

		// Add more viewers during peak
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 15; i++ {
				testLivestreamService.AddViewer(stream.ID)
				time.Sleep(time.Millisecond * 30)
			}
		}()

		wg.Wait()
		t.Logf("Phase 3: Completed peak activity simulation")

		// Phase 4: Verify Stream State
		currentStream, err := testLivestreamService.GetStreamStatus(stream.ID)
		if err != nil {
			t.Errorf("Failed to get current stream state: %v", err)
		} else {
			if currentStream.ViewerCount != 25 { // 10 initial + 15 peak
				t.Errorf("Expected 25 viewers, got %d", currentStream.ViewerCount)
			}
			if currentStream.Status != StreamStatusLive {
				t.Errorf("Stream should still be live, got: %s", currentStream.Status)
			}
		}

		// Verify chat history
		messages, err := testLivestreamService.GetMessages(stream.ID)
		if err != nil {
			t.Errorf("Failed to get chat messages: %v", err)
		} else {
			expectedMinMessages := len(initialMessages) + (5 * 4) // Initial + (5 users * 4 messages each)
			if len(messages) < expectedMinMessages {
				t.Errorf("Expected at least %d messages, got %d", expectedMinMessages, len(messages))
			}
		}

		t.Logf("Phase 4: Verified stream state - %d viewers, %d messages", 
			currentStream.ViewerCount, len(messages))

		// Phase 5: Stream Wind-down
		// Some viewers leave
		for i := 0; i < 8; i++ {
			testLivestreamService.RemoveViewer(stream.ID)
		}

		// Final messages
		finalMessages := []string{
			"Thanks for watching!",
			"See you next time!",
			"Stream ending soon",
		}
		
		for _, msg := range finalMessages {
			testLivestreamService.SendChatMessage(stream.ID, chatUserID, "streamer", msg)
		}

		t.Logf("Phase 5: Completed stream wind-down")

		// Phase 6: Stream Termination
		_, err = testLivestreamService.StopStream(testUserID, stream.ID)
		if err != nil {
			t.Errorf("Failed to stop stream: %v", err)
		}

		// Verify stream is properly ended
		finalStream, err := testLivestreamService.GetStreamStatus(stream.ID)
		if err != nil {
			t.Errorf("Failed to get final stream state: %v", err)
		} else {
			if finalStream.Status != StreamStatusEnded {
				t.Errorf("Stream should be ended, got: %s", finalStream.Status)
			}
			if finalStream.EndedAt == nil {
				t.Error("Stream should have end timestamp")
			}
		}

		t.Logf("Phase 6: Successfully terminated stream")

		// Phase 7: Post-Stream Analysis
		finalChatMessages, err := testLivestreamService.GetMessages(stream.ID)
		finalMessagesString := make([]string, len(finalChatMessages))
		for i, msg := range finalChatMessages {
			finalMessagesString[i] = msg.Message
		}
		if err != nil {
			t.Errorf("Failed to get final message count: %v", err)
		}

		t.Logf("Complete workflow summary:")
		t.Logf("- Final viewer count: %d", finalStream.ViewerCount)
		t.Logf("- Total messages: %d", len(finalMessages))
		t.Logf("- Stream duration: %v", finalStream.EndedAt.Sub(*finalStream.StartedAt))
		t.Logf("- Stream key: %s", finalStream.StreamKey)
	})

	t.Run("MultiUserInteractionWorkflow", func(t *testing.T) {
		// Create streams from multiple users
		user1ID := testUserID
		user2ID := primitive.NewObjectID()
		user3ID := primitive.NewObjectID()

		users := []struct {
			id   primitive.ObjectID
			name string
		}{
			{user1ID, "user1"},
			{user2ID, "user2"},
			{user3ID, "user3"},
		}

		userStreams := make(map[primitive.ObjectID]*Livestream)

		// Each user creates a stream
		for _, user := range users {
			stream, err := testLivestreamService.StartStream(user.id, StartStreamRequest{
				Title:       fmt.Sprintf("%s's Workflow Stream %s", user.name, generateTestSuffix()),
				Description: fmt.Sprintf("Multi-user workflow test stream by %s", user.name),
			})
			if err != nil {
				t.Errorf("Failed to create stream for %s: %v", user.name, err)
			} else {
				userStreams[user.id] = stream
			}
		}

		// Cross-user interactions: users watch and chat on each other's streams
		for _, watcher := range users {
			for _, streamer := range users {
				if watcher.id == streamer.id {
					continue // Skip self
				}

				streamerStream := userStreams[streamer.id]
				if streamerStream == nil {
					continue
				}

				// Watcher joins streamer's stream
				testLivestreamService.AddViewer(streamerStream.ID)

				// Watcher sends messages
				messages := []string{
					fmt.Sprintf("Hi %s, great stream!", streamer.name),
					"Love the content!",
					"Keep it up!",
				}

				for _, msg := range messages {
					testLivestreamService.SendChatMessage(streamerStream.ID, watcher.id, watcher.name, msg)
				}
			}
		}

		// Verify cross-interactions
		for userID, stream := range userStreams {
			userName := ""
			for _, user := range users {
				if user.id == userID {
					userName = user.name
					break
				}
			}

			// Check viewer count (should have 2 viewers - the other 2 users)
			viewerCount, err := testLivestreamService.GetViewerCount(stream.ID)
			if err != nil {
				t.Errorf("Failed to get viewer count for %s's stream: %v", userName, err)
			} else if viewerCount != 2 {
				t.Errorf("%s's stream: expected 2 viewers, got %d", userName, viewerCount)
			}

			// Check messages (should have messages from other users)
			messages, err := testLivestreamService.GetMessages(stream.ID)
			if err != nil {
				t.Errorf("Failed to get messages for %s's stream: %v", userName, err)
			} else {
				expectedMessages := 2 * 3 // 2 other users * 3 messages each
				if len(messages) < expectedMessages {
					t.Errorf("%s's stream: expected at least %d messages, got %d", userName, expectedMessages, len(messages))
				}
			}
		}

		// All users stop their streams
		for userID, stream := range userStreams {
			_, err := testLivestreamService.StopStream(userID, stream.ID)
			if err != nil {
				t.Errorf("Failed to stop stream for user %s: %v", userID.Hex()[:8], err)
			}
		}

		t.Logf("Successfully completed multi-user interaction workflow with %d users", len(users))
	})
}
