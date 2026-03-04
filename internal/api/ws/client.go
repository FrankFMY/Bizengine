// Package ws provides WebSocket hub and client for real-time event delivery.
package ws

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

const (
	writeTimeout  = 10 * time.Second
	pongTimeout   = 60 * time.Second
	pingInterval  = 30 * time.Second
	maxMessageSize = 4096
	sendBufSize    = 64
)

// Client represents a single WebSocket connection.
type Client struct {
	hub         *Hub
	conn        *websocket.Conn
	send        chan []byte
	workspaceID uuid.UUID
	userID      uuid.UUID

	mu      sync.RWMutex
	filters []string
	allEvts bool
}

// ClientMessage is a message from client to server.
type ClientMessage struct {
	Action  string   `json:"action"`
	Token   string   `json:"token,omitempty"`
	Filters []string `json:"filters,omitempty"`
}

// ServerMessage is a message from server to client.
type ServerMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

func newClient(hub *Hub, conn *websocket.Conn, wsID, userID uuid.UUID) *Client {
	return &Client{
		hub:         hub,
		conn:        conn,
		send:        make(chan []byte, sendBufSize),
		workspaceID: wsID,
		userID:      userID,
	}
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongTimeout))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Warn().Err(err).Msg("ws read error")
			}
			return
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			c.sendMsg(ServerMessage{Type: "error", Payload: map[string]string{"code": "INVALID_MESSAGE", "message": "invalid json"}})
			continue
		}

		c.handleMessage(msg)
	}
}

// writePump writes messages to the WebSocket connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleMessage(msg ClientMessage) {
	switch msg.Action {
	case "subscribe":
		c.mu.Lock()
		c.filters = append(c.filters, msg.Filters...)
		c.allEvts = false
		c.mu.Unlock()

	case "unsubscribe":
		c.mu.Lock()
		newFilters := make([]string, 0, len(c.filters))
		removeSet := make(map[string]bool, len(msg.Filters))
		for _, f := range msg.Filters {
			removeSet[f] = true
		}
		for _, f := range c.filters {
			if !removeSet[f] {
				newFilters = append(newFilters, f)
			}
		}
		c.filters = newFilters
		c.mu.Unlock()

	case "subscribe_all":
		c.mu.Lock()
		c.allEvts = true
		c.filters = nil
		c.mu.Unlock()

	case "ping":
		c.sendMsg(ServerMessage{Type: "pong"})
	}
}

// matchesEvent checks if the client should receive this event type.
func (c *Client) matchesEvent(eventType string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.allEvts {
		return true
	}

	if len(c.filters) == 0 {
		return false
	}

	for _, f := range c.filters {
		if f == eventType {
			return true
		}
		// Wildcard: "warehouse.stock.*" matches "warehouse.stock.received"
		if strings.HasSuffix(f, "*") {
			prefix := strings.TrimSuffix(f, "*")
			if strings.HasPrefix(eventType, prefix) {
				return true
			}
		}
	}

	return false
}

func (c *Client) sendMsg(msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.send <- data:
	default:
		// Buffer full, close connection
		close(c.send)
	}
}
