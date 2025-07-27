package livestream

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/websocket/v2"
	"github.com/pion/webrtc/v3"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// WebSocketMessage defines the structure for messages sent over WebSocket.
type WebSocketMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// Client represents a connected WebSocket client.
type Client struct {
	conn     *websocket.Conn
	send     chan []byte
	userID   primitive.ObjectID
	streamID primitive.ObjectID
}

// WebSocketHub manages all active clients and broadcasts messages.
type WebSocketHub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// NewWebSocketHub creates a new WebSocketHub.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's message processing loops.
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("WebSocket: Client registered (UserID: %s)", client.userID.Hex())

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("WebSocket: Client unregistered (UserID: %s)", client.userID.Hex())

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// WebSocketHandler provides the HTTP handler for WebSocket connections.
type WebSocketHandler struct {
	hub               *WebSocketHub
	livestreamService *LivestreamService
	webRTCManager     *WebRTCManager
}

// NewWebSocketHandler creates a new WebSocketHandler.
func NewWebSocketHandler(hub *WebSocketHub, ls *LivestreamService, wm *WebRTCManager) *WebSocketHandler {
	return &WebSocketHandler{
		hub:               hub,
		livestreamService: ls,
		webRTCManager:     wm,
	}
}

// ServeHTTP handles the WebSocket upgrade and connection lifecycle.
func (wh *WebSocketHandler) ServeHTTP(c *websocket.Conn) {
	userID, ok := c.Locals("user_id").(primitive.ObjectID)
	if !ok {
		log.Println("WebSocket: Unauthorized connection attempt.")
		c.Close()
		return
	}

	streamID, err := primitive.ObjectIDFromHex(c.Params("streamID"))
	if err != nil {
		log.Printf("WebSocket: Invalid stream ID: %v", err)
		c.Close()
		return
	}

	client := &Client{
		conn:     c,
		send:     make(chan []byte, 256),
		userID:   userID,
		streamID: streamID,
	}
	wh.hub.register <- client

	go client.writePump()
	client.readPump(wh)
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *Client) readPump(wh *WebSocketHandler) {
	defer func() {
		wh.hub.unregister <- c
		c.conn.Close()
	}()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("WebSocket: read error: %v", err)
			break
		}

		var msg WebSocketMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("WebSocket: error unmarshaling message: %v", err)
			continue
		}

		// Route the message based on its type
		switch msg.Type {
		case "chat_message":
			// Handle chat message payload
			var chatPayload struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(msg.Payload, &chatPayload); err != nil {
				log.Printf("WebSocket: error unmarshaling chat payload: %v", err)
				continue
			}
			// In a real app, you'd get the username from a user service
			wh.livestreamService.SendChatMessage(c.streamID, c.userID, "username", chatPayload.Message)
			// Broadcast the message to other clients in the same stream.
			// This part needs more logic to target specific streams.
			wh.hub.broadcast <- message

		case "webrtc_offer":
			var offer webrtc.SessionDescription
			if err := json.Unmarshal(msg.Payload, &offer); err != nil {
				log.Printf("WebSocket: error unmarshaling webrtc_offer payload: %v", err)
				continue
			}
			answer, err := wh.webRTCManager.HandleOffer(offer, c.userID.Hex(), c.streamID.Hex())
			if err != nil {
				log.Printf("WebSocket: error handling webrtc_offer: %v", err)
				continue
			}
			answerBytes, _ := json.Marshal(answer)
			response := WebSocketMessage{Type: "webrtc_answer", Payload: answerBytes}
			responseBytes, _ := json.Marshal(response)
			c.send <- responseBytes

		case "webrtc_ice_candidate":
			var candidate webrtc.ICECandidateInit
			if err := json.Unmarshal(msg.Payload, &candidate); err != nil {
				log.Printf("WebSocket: error unmarshaling webrtc_ice_candidate payload: %v", err)
				continue
			}
			wh.webRTCManager.HandleICECandidate(candidate, c.userID.Hex())

		default:
			log.Printf("WebSocket: Unknown message type: %s", msg.Type)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *Client) writePump() {
	defer c.conn.Close()
	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			log.Printf("WebSocket: write error: %v", err)
			return
		}
	}
}
