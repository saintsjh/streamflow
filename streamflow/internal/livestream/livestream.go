package livestream

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StreamStatus string

const (
	StreamStatusOffline StreamStatus = "OFFLINE"
	StreamStatusLive    StreamStatus = "LIVE"
	StreamStatusEnded   StreamStatus = "ENDED"
)

type Livestream struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	UserID      primitive.ObjectID `bson:"user_id"`
	Title       string             `bson:"title"`
	Description string             `bson:"description"`
	Status      StreamStatus       `bson:"status"`
	StreamKey   string             `bson:"stream_key"`
	ViewerCount int                `bson:"viewer_count"`
	StartedAt   *time.Time         `bson:"started_at,omitempty"`
	EndedAt     *time.Time         `bson:"ended_at,omitempty"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
}

type StartStreamRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}