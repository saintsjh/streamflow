package livestream

import (
	"os/exec"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RecorderService struct {
	ffmpegService        *FFmpegService
	storagePath          string
	recordings           map[string]*RecorderSession
	recordingsCollection *mongo.Collection
	mu                   sync.RWMutex
}

type RecorderSession struct {
	StreamID    primitive.ObjectID `bson:"stream_id"`
	OutputPath  string             `bson:"output_path"`
	StartTime   time.Time          `bson:"start_time"`
	IsRecording bool               `bson:"is_recording"`
	Process     *exec.Cmd          `bson:"-"`
}
