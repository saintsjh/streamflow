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
	Duration    float64 `bson:"duration" json:"Duration"`         // Duration in seconds
	Width       int     `bson:"width" json:"Width"`               // Video width in pixels
	Height      int     `bson:"height" json:"Height"`             // Video height in pixels
	Codec       string  `bson:"codec" json:"Codec"`               // Video codec (e.g., h264, h265)
	AudioCodec  string  `bson:"audio_codec" json:"AudioCodec"`    // Audio codec (e.g., aac, mp3)
	Bitrate     int     `bson:"bitrate" json:"Bitrate"`           // Video bitrate in kbps
	FrameRate   float64 `bson:"frame_rate" json:"FrameRate"`      // Frames per second
	FileSize    int64   `bson:"file_size" json:"FileSize"`        // Original file size in bytes
}

type Video struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"ID"`
	Title       string             `bson:"title" json:"Title"`
	Description string             `bson:"description" json:"Description"`
	Status      VideoStatus        `bson:"status" json:"Status"`
	CreatedAt   time.Time          `bson:"created_at" json:"CreatedAt"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"UpdatedAt"`
	UserID      primitive.ObjectID `bson:"user_id" json:"UserID"`
	ViewCount   int64              `bson:"view_count" json:"ViewCount"`
	FilePath    string             `bson:"file_path" json:"FilePath"`         // Path to original uploaded file
	HLSPath     string             `bson:"hls_path" json:"HLSPath"`           // Path to HLS playlist
	ThumbnailPath string           `bson:"thumbnail_path" json:"ThumbnailPath"` // Path to thumbnail image
	Metadata    VideoMetadata      `bson:"metadata" json:"Metadata"`          // Video metadata
	Error       string             `bson:"error,omitempty" json:"Error,omitempty"` // Error message if processing failed
}
