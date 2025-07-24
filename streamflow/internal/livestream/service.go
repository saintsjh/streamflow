package livestream

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type LivestreamService struct {
	livestreamCollection *mongo.Collection
	chatCollection       *mongo.Collection
}

func NewLiveStreamService(db *mongo.Database) *LivestreamService {
	return &LivestreamService{
		livestreamCollection: db.Collection("livestreams"),
		chatCollection:       db.Collection("chat_messages"),
	}
}

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

func (s *LivestreamService) GetStreamStatus(streamID primitive.ObjectID) (*Livestream, error) {
	var livestream *Livestream
	if err := s.livestreamCollection.FindOne(context.Background(), bson.M{"_id": streamID}).Decode(&livestream); err != nil {
		return nil, err
	}

	return livestream, nil
}

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

func (s *LivestreamService) GetMessages(streamID primitive.ObjectID) ([]*ChatMessage, error) {
	// get all chat messages for a stream matching the streamID
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

func (s *LivestreamService) SaveChatMessage(message *ChatMessage) error {
	_, err := s.chatCollection.InsertOne(context.Background(), message)
	if err != nil {
		return fmt.Errorf("failed to save chat message: %w", err)
	}
	return nil
}

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

func generateStreamKey() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
