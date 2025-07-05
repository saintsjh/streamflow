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

type Video struct {
	ID primitive.ObjectID `bson:"_id,omitempty"`
	Title string `bson:"title"`
	Description string `bson:"description"`
	Status VideoStatus `bson:"status"`
	CreatedAt time.Time `bson:"created_at"`
	UpdatedAt time.Time `bson:"updated_at"`
}
