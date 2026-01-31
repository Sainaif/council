package websocket

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gofiber/contrib/websocket"
)

type Message struct {
	SessionID string      `json:"session_id"`
	Event     string      `json:"event"`
	Data      interface{} `json:"data"`
}

type Client struct {
	Conn      *websocket.Conn
	SessionID string
	Send      chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	sessions   map[string]map[*Client]bool
	broadcast  chan *Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
	shutdown   chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		sessions:   make(map[string]map[*Client]bool),
		broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		shutdown:   make(chan struct{}),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			if h.sessions[client.SessionID] == nil {
				h.sessions[client.SessionID] = make(map[*Client]bool)
			}
			h.sessions[client.SessionID][client] = true
			h.mu.Unlock()
			log.Printf("Client connected to session %s", client.SessionID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				if h.sessions[client.SessionID] != nil {
					delete(h.sessions[client.SessionID], client)
					if len(h.sessions[client.SessionID]) == 0 {
						delete(h.sessions, client.SessionID)
					}
				}
				close(client.Send)
			}
			h.mu.Unlock()
			log.Printf("Client disconnected from session %s", client.SessionID)

		case message := <-h.broadcast:
			h.mu.RLock()
			clients := h.sessions[message.SessionID]
			h.mu.RUnlock()

			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error marshaling message: %v", err)
				continue
			}

			for client := range clients {
				select {
				case client.Send <- data:
				default:
					h.unregister <- client
				}
			}

		case <-h.shutdown:
			h.mu.Lock()
			for client := range h.clients {
				close(client.Send)
			}
			h.clients = make(map[*Client]bool)
			h.sessions = make(map[string]map[*Client]bool)
			h.mu.Unlock()
			return
		}
	}
}

func (h *Hub) Shutdown() {
	close(h.shutdown)
}

func (h *Hub) HandleConnection(c *websocket.Conn, sessionID string) {
	client := &Client{
		Conn:      c,
		SessionID: sessionID,
		Send:      make(chan []byte, 256),
	}

	h.register <- client

	// Writer goroutine
	go func() {
		defer func() {
			_ = c.Close()
		}()
		for message := range client.Send {
			if err := c.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		}
	}()

	// Reader goroutine (mainly for keeping connection alive)
	defer func() {
		h.unregister <- client
		_ = c.Close()
	}()

	for {
		_, _, err := c.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// Broadcast sends a message to all clients in a session
func (h *Hub) Broadcast(sessionID, event string, data interface{}) {
	h.broadcast <- &Message{
		SessionID: sessionID,
		Event:     event,
		Data:      data,
	}
}

// BroadcastToSession sends an event to all clients watching a specific session
func (h *Hub) BroadcastToSession(sessionID string, msg *Message) {
	msg.SessionID = sessionID
	h.broadcast <- msg
}

// Event constants
const (
	EventCouncilStarted     = "council.started"
	EventModelResponding    = "model.responding"
	EventModelResponseChunk = "model.response_chunk"
	EventModelComplete      = "model.complete"
	EventVotingStarted      = "voting.started"
	EventVoteReceived       = "voting.received"
	EventSynthesisStarted   = "synthesis.started"
	EventSynthesisComplete  = "synthesis.complete"
	EventCouncilCompleted   = "council.completed"
	EventCouncilFailed      = "council.failed"
	EventError              = "error"
)
