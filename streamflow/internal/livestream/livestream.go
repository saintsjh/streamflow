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
	ID                 primitive.ObjectID `bson:"_id,omitempty"`
	UserID             primitive.ObjectID `bson:"user_id"`
	Title              string             `bson:"title"`
	Description        string             `bson:"description"`
	Status             StreamStatus       `bson:"status"`
	StreamKey          string             `bson:"stream_key"`
	ViewerCount        int                `bson:"viewer_count"`
	PeakViewerCount    int                `bson:"peak_viewer_count"`
	AverageViewerCount int                `bson:"average_viewer_count"`
	StartedAt          *time.Time         `bson:"started_at,omitempty"`
	EndedAt            *time.Time         `bson:"ended_at,omitempty"`
	CreatedAt          time.Time          `bson:"created_at"`
	UpdatedAt          time.Time          `bson:"updated_at"`
}

type StartStreamRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type ChatCollection struct {
	LivestreamID primitive.ObjectID `bson:"livestream_id"`
	Messages     []*ChatMessage     `bson:"messages"`
	CreatedAt    time.Time          `bson:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at"`
}

type Recording struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	StreamID  primitive.ObjectID `bson:"stream_id"`
	FilePath  string             `bson:"file_path"`
	Duration  time.Duration      `bson:"duration"`
	FileSize  int64              `bson:"file_size"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}

type StreamAnalytics struct {
	StreamID       primitive.ObjectID `bson:"stream_id"`
	ViewerCount    int                `bson:"viewer_count"`
	ChatCount      int                `bson:"chat_count"`
	Duration       time.Duration      `bson:"duration"`
	PeakViewers    int                `bson:"peak_viewers"`
	AverageViewers int                `bson:"average_viewers"`
}
