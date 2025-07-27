package livestream

import (
	"errors"
	"log"
	"sync"

	"github.com/pion/webrtc/v3"
)

// WebRTCManager manages all active WebRTC peer connections.
type WebRTCManager struct {
	api             *webrtc.API
	peerConnections map[string]*webrtc.PeerConnection // Map of viewerID to PeerConnection
	mu              sync.RWMutex
	streamManager   *StreamManager
}

// NewWebRTCManager creates a new WebRTC manager.
func NewWebRTCManager(sm *StreamManager) (*WebRTCManager, error) {
	m := &webrtc.MediaEngine{}
	if err := m.RegisterDefaultCodecs(); err != nil {
		return nil, err
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m))
	return &WebRTCManager{
		api:             api,
		peerConnections: make(map[string]*webrtc.PeerConnection),
		streamManager:   sm,
	}, nil
}

// HandleOffer processes an SDP offer from a client and returns an answer.
func (wm *WebRTCManager) HandleOffer(offer webrtc.SessionDescription, viewerID, streamKey string) (*webrtc.SessionDescription, error) {
	peerConnection, err := wm.api.NewPeerConnection(webrtc.Configuration{})
	if err != nil {
		log.Printf("WebRTC: Failed to create PeerConnection: %v", err)
		return nil, err
	}

	wm.addPeerConnection(viewerID, peerConnection)

	// Get the existing tracks from the stream manager.
	videoTrack, audioTrack := wm.streamManager.GetStreamTracks(streamKey)
	if videoTrack == nil || audioTrack == nil {
		return nil, errors.New("stream is not currently active or does not have media tracks")
	}

	// Add the video and audio tracks to the peer connection.
	if _, err := peerConnection.AddTrack(videoTrack); err != nil {
		return nil, err
	}
	if _, err := peerConnection.AddTrack(audioTrack); err != nil {
		return nil, err
	}

	// Set the remote SessionDescription
	if err := peerConnection.SetRemoteDescription(offer); err != nil {
		return nil, err
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		return nil, err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err := peerConnection.SetLocalDescription(answer); err != nil {
		return nil, err
	}

	log.Printf("WebRTC: PeerConnection created for viewer %s, attached to stream %s", viewerID, streamKey)
	return &answer, nil
}

// HandleICECandidate adds a new ICE candidate from the client.
func (wm *WebRTCManager) HandleICECandidate(candidate webrtc.ICECandidateInit, viewerID string) error {
	wm.mu.RLock()
	pc, exists := wm.peerConnections[viewerID]
	wm.mu.RUnlock()

	if !exists {
		return nil // Or return an error
	}

	return pc.AddICECandidate(candidate)
}

// addPeerConnection safely adds a new peer connection to the map.
func (wm *WebRTCManager) addPeerConnection(viewerID string, pc *webrtc.PeerConnection) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.peerConnections[viewerID] = pc
}

// ClosePeerConnection closes and removes a peer connection.
func (wm *WebRTCManager) ClosePeerConnection(viewerID string) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if pc, exists := wm.peerConnections[viewerID]; exists {
		pc.Close()
		delete(wm.peerConnections, viewerID)
		log.Printf("WebRTC: Closed PeerConnection for viewer %s", viewerID)
	}
}
