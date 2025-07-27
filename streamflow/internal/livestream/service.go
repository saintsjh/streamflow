package livestream

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type LivestreamService struct {
	livestreamCollection *mongo.Collection
	chatCollection       *mongo.Collection
	recorderService      *RecorderService
}

// NewLiveStreamService creates a new livestream service with database collections
func NewLiveStreamService(db *mongo.Database) *LivestreamService {
	return &LivestreamService{
		livestreamCollection: db.Collection("livestreams"),
		chatCollection:       db.Collection("chat_messages"),
		recorderService:      NewRecorderService("./storage/recordings", db),
	}
}

// StartStream creates a new livestream entry in the database
func (s *LivestreamService) StartStream(userID primitive.ObjectID, req StartStreamRequest) (*Livestream, error) {
	streamKey := generateStreamKey()
	now := time.Now()
	livestream := &Livestream{
		ID:          primitive.NewObjectID(),
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		Status:      StreamStatusLive,
		StreamKey:   streamKey,
		ViewerCount: 0,
		StartedAt:   &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := s.livestreamCollection.InsertOne(context.Background(), livestream)
	if err != nil {
		return nil, err
	}

	return livestream, nil
}

// StopStream updates a livestream status to ended
func (s *LivestreamService) StopStream(userID primitive.ObjectID, streamID primitive.ObjectID) (*Livestream, error) {
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"status":    StreamStatusEnded,
			"endedAt":   now,
			"updatedAt": now,
		},
	}
	result, err := s.livestreamCollection.UpdateOne(context.Background(),
		bson.M{"_id": streamID, "user_id": userID},
		update)
	if err != nil {
		return nil, fmt.Errorf("failed to stop stream: %w", err)
	}

	if result.MatchedCount == 0 {
		return nil, fmt.Errorf("stream not found or unauthorized")
	}

	return nil, nil
}

// GetStreamStatus retrieves the current status of a livestream
func (s *LivestreamService) GetStreamStatus(streamID primitive.ObjectID) (*Livestream, error) {
	var livestream *Livestream
	if err := s.livestreamCollection.FindOne(context.Background(), bson.M{"_id": streamID}).Decode(&livestream); err != nil {
		return nil, err
	}

	return livestream, nil
}

// ListStreams returns all currently live streams
func (s *LivestreamService) ListStreams() ([]*Livestream, error) {
	cursor, err := s.livestreamCollection.Find(context.Background(), bson.M{"status": StreamStatusLive})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var streams []*Livestream
	if err := cursor.All(context.Background(), &streams); err != nil {
		return streams, nil
	}

	return streams, nil
}

// GetMessages retrieves all chat messages for a specific stream
func (s *LivestreamService) GetMessages(streamID primitive.ObjectID) ([]*ChatMessage, error) {
	cursor, err := s.chatCollection.Find(context.Background(), bson.M{"stream_id": streamID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var messages []*ChatMessage
	if err := cursor.All(context.Background(), &messages); err != nil {
		return nil, err
	}
	return messages, nil
}

// SaveChatMessage persists a chat message to the database
func (s *LivestreamService) SaveChatMessage(message *ChatMessage) error {
	_, err := s.chatCollection.InsertOne(context.Background(), message)
	if err != nil {
		return fmt.Errorf("failed to save chat message: %w", err)
	}
	return nil
}

// SendChatMessage creates and saves a new chat message
func (s *LivestreamService) SendChatMessage(streamID primitive.ObjectID, userID primitive.ObjectID, userName, message string) error {
	chatMessage := &ChatMessage{
		ID:        primitive.NewObjectID(),
		StreamID:  streamID,
		UserID:    userID,
		UserName:  userName,
		Message:   message,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	err := s.SaveChatMessage(chatMessage)
	if err != nil {
		return fmt.Errorf("failed to send chat message: %w", err)
	}
	return nil
}

// generateStreamKey creates a unique stream key for RTMP authentication
func generateStreamKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// NewRecorderService creates a new recorder service for video recording
func NewRecorderService(storagePath string, db *mongo.Database) *RecorderService {
	return &RecorderService{
		storagePath:          storagePath,
		recordings:           make(map[string]*RecorderSession),
		recordingsCollection: db.Collection("recordings"),
	}
}

// StartRecording begins recording a livestream using FFmpeg
func (r *RecorderService) StartRecording(streamID primitive.ObjectID, rtmpURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	outputPath := fmt.Sprintf("%s/stream_%s_%s.mp4",
		r.storagePath, streamID.Hex(), time.Now().Format("20060102_150405"))

	args := []string{
		"-i", rtmpURL,
		"-c", "copy",
		"-f", "mp4",
		"-movflags", "frag_keyframe+empty_moov",
		outputPath,
	}

	cmd := exec.Command("ffmpeg", args...)
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	r.recordings[streamID.Hex()] = &RecorderSession{
		StreamID:    streamID,
		OutputPath:  outputPath,
		StartTime:   time.Now(),
		IsRecording: true,
		Process:     cmd,
	}

	return nil
}

// StopRecording gracefully stops the FFmpeg recording process
func (r *RecorderService) StopRecording(streamID primitive.ObjectID) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, exists := r.recordings[streamID.Hex()]
	if !exists {
		return fmt.Errorf("no active recording for stream %s", streamID.Hex())
	}

	if session.Process != nil && session.Process.Process != nil {
		session.Process.Process.Signal(os.Interrupt)
		session.Process.Wait()
	}

	session.IsRecording = false
	delete(r.recordings, streamID.Hex())

	return nil
}

// GetRecordingStatus returns the current recording session status
func (r *RecorderService) GetRecordingStatus(streamID primitive.ObjectID) (*RecorderSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, exists := r.recordings[streamID.Hex()]
	if !exists {
		return nil, fmt.Errorf("no recording session found")
	}

	return session, nil
}

// GetStreamByKey retrieves a stream by its stream key
func (s *LivestreamService) GetStreamByKey(streamKey string) (*Livestream, error) {
	var livestream Livestream
	err := s.livestreamCollection.FindOne(context.Background(), bson.M{"stream_key": streamKey}).Decode(&livestream)
	if err != nil {
		return nil, err
	}
	return &livestream, nil
}

// UpdateStream updates stream metadata
func (s *LivestreamService) UpdateStream(streamID primitive.ObjectID, updates map[string]interface{}) error {
	updates["updatedAt"] = time.Now()
	update := bson.M{"$set": updates}

	result, err := s.livestreamCollection.UpdateOne(context.Background(),
		bson.M{"_id": streamID}, update)
	if err != nil {
		return fmt.Errorf("failed to update stream: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("stream not found")
	}

	return nil
}

// GetUserStreams returns all streams created by a specific user
func (s *LivestreamService) GetUserStreams(userID primitive.ObjectID) ([]*Livestream, error) {
	cursor, err := s.livestreamCollection.Find(context.Background(), bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var streams []*Livestream
	if err := cursor.All(context.Background(), &streams); err != nil {
		return nil, err
	}
	return streams, nil
}

// DeleteStream removes a stream from the database
func (s *LivestreamService) DeleteStream(streamID primitive.ObjectID) error {
	result, err := s.livestreamCollection.DeleteOne(context.Background(), bson.M{"_id": streamID})
	if err != nil {
		return fmt.Errorf("failed to delete stream: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("stream not found")
	}

	return nil
}

// AddViewer increments the viewer count for a stream
func (s *LivestreamService) AddViewer(streamID primitive.ObjectID) error {
	update := bson.M{"$inc": bson.M{"viewer_count": 1}}

	result, err := s.livestreamCollection.UpdateOne(context.Background(),
		bson.M{"_id": streamID}, update)
	if err != nil {
		return fmt.Errorf("failed to add viewer: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("stream not found")
	}

	return nil
}

// RemoveViewer decrements the viewer count for a stream
func (s *LivestreamService) RemoveViewer(streamID primitive.ObjectID) error {
	update := bson.M{"$inc": bson.M{"viewer_count": -1}}

	result, err := s.livestreamCollection.UpdateOne(context.Background(),
		bson.M{"_id": streamID}, update)
	if err != nil {
		return fmt.Errorf("failed to remove viewer: %w", err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("stream not found")
	}

	return nil
}

// GetViewerCount returns the current viewer count for a stream
func (s *LivestreamService) GetViewerCount(streamID primitive.ObjectID) (int, error) {
	var livestream Livestream
	err := s.livestreamCollection.FindOne(context.Background(), bson.M{"_id": streamID}).Decode(&livestream)
	if err != nil {
		return 0, err
	}

	return livestream.ViewerCount, nil
}

// SearchStreams finds streams matching the search query
func (s *LivestreamService) SearchStreams(query string) ([]*Livestream, error) {
	filter := bson.M{
		"$and": []bson.M{
			{"status": StreamStatusLive},
			{"$or": []bson.M{
				{"title": bson.M{"$regex": query, "$options": "i"}},
				{"description": bson.M{"$regex": query, "$options": "i"}},
			}},
		},
	}

	cursor, err := s.livestreamCollection.Find(context.Background(), filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var streams []*Livestream
	if err := cursor.All(context.Background(), &streams); err != nil {
		return nil, err
	}
	return streams, nil
}

// GetPopularStreams returns streams ordered by viewer count
func (s *LivestreamService) GetPopularStreams(limit int) ([]*Livestream, error) {
	opts := options.Find().SetSort(bson.D{{Key: "viewer_count", Value: -1}}).SetLimit(int64(limit))

	cursor, err := s.livestreamCollection.Find(context.Background(), bson.M{"status": StreamStatusLive}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var streams []*Livestream
	if err := cursor.All(context.Background(), &streams); err != nil {
		return nil, err
	}
	return streams, nil
}

// GetStreamRecordings returns all recordings for a specific stream
func (s *LivestreamService) GetStreamRecordings(streamID primitive.ObjectID) ([]*Recording, error) {
	cursor, err := s.recorderService.recordingsCollection.Find(context.Background(), bson.M{"stream_id": streamID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(context.Background())

	var recordings []*Recording
	if err := cursor.All(context.Background(), &recordings); err != nil {
		return nil, err
	}
	return recordings, nil
}

// DeleteRecording removes a recording from storage and database
func (s *LivestreamService) DeleteRecording(recordingID primitive.ObjectID) error {
	var recording Recording
	err := s.recorderService.recordingsCollection.FindOne(context.Background(), bson.M{"_id": recordingID}).Decode(&recording)
	if err != nil {
		return fmt.Errorf("recording not found: %w", err)
	}

	// Delete file from storage
	if err := os.Remove(recording.FilePath); err != nil {
		return fmt.Errorf("failed to delete recording file: %w", err)
	}

	// Delete from database
	result, err := s.recorderService.recordingsCollection.DeleteOne(context.Background(), bson.M{"_id": recordingID})
	if err != nil {
		return fmt.Errorf("failed to delete recording from database: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("recording not found in database")
	}

	return nil
}

// GetStreamAnalytics returns analytics data for a stream
func (s *LivestreamService) GetStreamAnalytics(streamID primitive.ObjectID) (*StreamAnalytics, error) {
	stream, err := s.GetStreamStatus(streamID)
	if err != nil {
		return nil, err
	}

	// Get chat message count
	chatCount, err := s.chatCollection.CountDocuments(context.Background(), bson.M{"stream_id": streamID})
	if err != nil {
		return nil, err
	}

	// Get recording duration if stream has ended
	var duration time.Duration
	if stream.Status == StreamStatusEnded && stream.StartedAt != nil && stream.EndedAt != nil {
		duration = stream.EndedAt.Sub(*stream.StartedAt)
	}

	analytics := &StreamAnalytics{
		StreamID:       streamID,
		ViewerCount:    stream.ViewerCount,
		ChatCount:      int(chatCount),
		Duration:       duration,
		PeakViewers:    stream.PeakViewerCount,
		AverageViewers: stream.AverageViewerCount,
	}

	return analytics, nil
}
