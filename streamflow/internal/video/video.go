package video

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VideoStatus string

const (
	StatusPending VideoStatus = "PENDING"
	StatusProcessing VideoStatus = "PROCESSING"
	StatusCompleted VideoStatus = "COMPLETED"
	StatusFailed VideoStatus = "FAILED"
)

type VideoMetadata struct {
	Duration    float64 `bson:"duration"`     // Duration in seconds
	Width       int     `bson:"width"`        // Video width in pixels
	Height      int     `bson:"height"`       // Video height in pixels
	Codec       string  `bson:"codec"`        // Video codec (e.g., h264, h265)
	AudioCodec  string  `bson:"audio_codec"`  // Audio codec (e.g., aac, mp3)
	Bitrate     int     `bson:"bitrate"`      // Video bitrate in kbps
	FrameRate   float64 `bson:"frame_rate"`   // Frames per second
	FileSize    int64   `bson:"file_size"`    // Original file size in bytes
}

type Video struct {
	ID          primitive.ObjectID `bson:"_id,omitempty"`
	Title       string             `bson:"title"`
	Description string             `bson:"description"`
	Status      VideoStatus        `bson:"status"`
	CreatedAt   time.Time          `bson:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at"`
	UserID      primitive.ObjectID `bson:"user_id"`
	ViewCount   int64              `bson:"view_count"`
	FilePath    string             `bson:"file_path"`     // Path to original uploaded file
	HLSPath     string             `bson:"hls_path"`      // Path to HLS playlist
	ThumbnailPath string           `bson:"thumbnail_path"` // Path to thumbnail image
	Metadata    VideoMetadata      `bson:"metadata"`      // Video metadata
	Error       string             `bson:"error,omitempty"` // Error message if processing failed
}
