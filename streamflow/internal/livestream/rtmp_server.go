package livestream

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/yutopp/go-rtmp"
	"github.com/yutopp/go-rtmp/message"
)

type RTMPServer struct {
	server        *rtmp.Server
	port          string
	streamManager *StreamManager
}

type RTMPServerHandler struct {
	rtmp.DefaultHandler
	streamManager *StreamManager
	conn          net.Conn // Store the connection to identify it on close
	streamKey     string   // Add this field to store the key upon successful publish
}

// NewRTMPServer creates a new RTMP server instance
func NewRTMPServer(port string, sm *StreamManager) *RTMPServer {
	return &RTMPServer{
		port:          port,
		streamManager: sm,
	}
}

// Start initializes and starts the RTMP server
func (s *RTMPServer) Start() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		return err
	}

	config := &rtmp.ServerConfig{
		OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
			handler := &RTMPServerHandler{
				streamManager: s.streamManager,
				conn:          conn,
			}
			return conn, &rtmp.ConnConfig{
				Handler: handler,
			}
		},
	}
	s.server = rtmp.NewServer(config)

	log.Printf("Starting RTMP server on %s", listener.Addr())
	return s.server.Serve(listener)
}

func (h *RTMPServerHandler) OnPublish(ctx *rtmp.StreamContext, timestamp uint32, cmd *message.NetStreamPublish) error {
	streamKey := cmd.PublishingName
	log.Printf("RTMP: Publish request for stream key: %s", streamKey)

	if streamKey == "" {
		return errors.New("rtmp: publishing name is required")
	}

	stream, err := h.streamManager.livestreamService.GetStreamByKey(streamKey)
	if err != nil || stream.Status != StreamStatusLive {
		return errors.New("rtmp: invalid or inactive stream key")
	}

	// Start stream management
	h.streamManager.HandleStreamStart(streamKey, stream.ID)

	// Get the tracks from the manager to verify they were created
	videoTrack, audioTrack := h.streamManager.GetStreamTracks(streamKey)
	if videoTrack == nil || audioTrack == nil {
		return errors.New("rtmp: failed to create tracks")
	}

	h.streamKey = streamKey
	return nil
}

func (h *RTMPServerHandler) OnVideo(timestamp uint32, reader io.Reader) error {
	if h.streamKey == "" {
		return nil // Not ready yet
	}

	// Read all data from the reader
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return h.streamManager.WriteVideoSample(h.streamKey, data, time.Second/30)
}

func (h *RTMPServerHandler) OnAudio(timestamp uint32, reader io.Reader) error {
	if h.streamKey == "" {
		return nil // Not ready yet
	}

	// Read all data from the reader
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	return h.streamManager.WriteAudioSample(h.streamKey, data, time.Millisecond*20)
}

func (h *RTMPServerHandler) OnPlay(ctx *rtmp.StreamContext, timestamp uint32, cmd *message.NetStreamPlay) error {
	streamKey := cmd.StreamName
	log.Printf("RTMP: Play request for stream key: %s", streamKey)

	// Delegate viewer join event to the manager.
	h.streamManager.HandleViewerJoin(streamKey)

	return nil
}

func (h *RTMPServerHandler) OnClose() {
	log.Printf("RTMP: Connection closed from %s", h.conn.RemoteAddr().String())

	// If a streamKey was associated with this connection, it means it was a publishing connection.
	if h.streamKey != "" {
		log.Printf("RTMP: Publisher for stream key '%s' has disconnected.", h.streamKey)
		// Delegate the end-of-stream logic to the manager.
		h.streamManager.HandleStreamEnd(h.streamKey)
	} else {
		log.Printf("RTMP: A viewer/non-publishing connection has closed.")
	}
}
