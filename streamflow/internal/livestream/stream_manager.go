package livestream

import (
	"log"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ActiveStream holds real-time data for a live stream.
type ActiveStream struct {
	StreamID     primitive.ObjectID
	StreamKey    string
	ViewerCount  int32
	IsHealthy    bool
	LastActivity time.Time
	VideoTrack   *webrtc.TrackLocalStaticSample
	AudioTrack   *webrtc.TrackLocalStaticSample
}

// StreamManager orchestrates all active livestreaming sessions.
type StreamManager struct {
	livestreamService *LivestreamService
	activeStreams     map[string]*ActiveStream
	mu                sync.RWMutex
}

// NewStreamManager creates a new stream manager.
func NewStreamManager(ls *LivestreamService) *StreamManager {
	return &StreamManager{
		livestreamService: ls,
		activeStreams:     make(map[string]*ActiveStream),
	}
}

// HandleStreamStart initializes stream management for a new publishing stream.
func (sm *StreamManager) HandleStreamStart(streamKey string, streamID primitive.ObjectID) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	log.Printf("StreamManager: Handling start for stream key: %s", streamKey)

	videoTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", streamKey)
	if err != nil {
		log.Printf("StreamManager: Error creating video track: %v", err)
		return
	}
	audioTrack, err := webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus}, "audio", streamKey)
	if err != nil {
		log.Printf("StreamManager: Error creating audio track: %v", err)
		return
	}

	sm.activeStreams[streamKey] = &ActiveStream{
		StreamID:     streamID,
		StreamKey:    streamKey,
		IsHealthy:    true,
		LastActivity: time.Now(),
		VideoTrack:   videoTrack,
		AudioTrack:   audioTrack,
	}

	log.Printf("StreamManager: Started and now managing stream %s", streamKey)
}

// HandleStreamEnd orchestrates cleanup when a stream stops.
func (sm *StreamManager) HandleStreamEnd(streamKey string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	log.Printf("StreamManager: Handling end for stream key: %s", streamKey)

	if stream, exists := sm.activeStreams[streamKey]; exists {
		// Stop the recording.
		go sm.livestreamService.recorderService.StopRecording(stream.StreamID)
		// Remove from active management.
		delete(sm.activeStreams, streamKey)
		log.Printf("StreamManager: Stopped and cleaned up stream %s", streamKey)
	}
}

// HandleViewerJoin updates the viewer count when a viewer starts watching.
func (sm *StreamManager) HandleViewerJoin(streamKey string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if stream, exists := sm.activeStreams[streamKey]; exists {
		stream.ViewerCount++
		go sm.livestreamService.AddViewer(stream.StreamID)
		log.Printf("StreamManager: Viewer joined stream %s. Total viewers: %d", streamKey, stream.ViewerCount)
	}
}

// HandleViewerLeave updates the viewer count when a viewer stops watching.
func (sm *StreamManager) HandleViewerLeave(streamKey string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if stream, exists := sm.activeStreams[streamKey]; exists {
		stream.ViewerCount--
		go sm.livestreamService.RemoveViewer(stream.StreamID)
		log.Printf("StreamManager: Viewer left stream %s. Total viewers: %d", streamKey, stream.ViewerCount)
	}
}

// GetStreamTracks returns the active video and audio tracks for a given stream key.
func (sm *StreamManager) GetStreamTracks(streamKey string) (*webrtc.TrackLocalStaticSample, *webrtc.TrackLocalStaticSample) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if stream, exists := sm.activeStreams[streamKey]; exists {
		return stream.VideoTrack, stream.AudioTrack
	}
	return nil, nil
}

// WriteVideoSample writes a video sample to the stream.
func (sm *StreamManager) WriteVideoSample(streamKey string, data []byte, duration time.Duration) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if stream, exists := sm.activeStreams[streamKey]; exists {
		return stream.VideoTrack.WriteSample(media.Sample{Data: data, Duration: duration})
	}
	return nil
}

// WriteAudioSample writes an audio sample to the stream.
func (sm *StreamManager) WriteAudioSample(streamKey string, data []byte, duration time.Duration) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if stream, exists := sm.activeStreams[streamKey]; exists {
		return stream.AudioTrack.WriteSample(media.Sample{Data: data, Duration: duration})
	}
	return nil
}
