package livestream

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatMessage struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	StreamID  primitive.ObjectID `bson:"stream_id"`
	UserID    primitive.ObjectID `bson:"user_id"`
	UserName  string             `bson:"user_name"`
	Message   string             `bson:"message"`
	CreatedAt time.Time          `bson:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"`
}
