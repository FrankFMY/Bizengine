package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/types"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins in development
	},
}

// Hub manages all WebSocket clients and broadcasts events.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	authSvc    *auth.Service
}

// NewHub creates a new WebSocket hub.
func NewHub(authSvc *auth.Service) *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		authSvc:    authSvc,
	}
}

// Run starts the hub's main loop. Call in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Debug().Str("workspace_id", client.workspaceID.String()).Msg("ws client connected")

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Debug().Str("workspace_id", client.workspaceID.String()).Msg("ws client disconnected")
		}
	}
}

// HandleEvent implements event.Subscriber to receive events from the Event Bus.
func (h *Hub) HandleEvent(ctx context.Context, ev types.Event) error {
	msg := ServerMessage{
		Type:    "event",
		Payload: ev,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if client.workspaceID != ev.WorkspaceID {
			continue
		}
		if !client.matchesEvent(ev.Type) {
			continue
		}
		select {
		case client.send <- data:
		default:
			// Slow client, will be cleaned up
		}
	}

	return nil
}

// ServeWS handles WebSocket upgrade requests.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	// Authenticate via query param
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "missing token", http.StatusUnauthorized)
		return
	}

	claims, err := h.authSvc.VerifyToken(tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		http.Error(w, "invalid user", http.StatusUnauthorized)
		return
	}

	var wsID uuid.UUID
	if claims.WsID != "" {
		wsID, err = uuid.Parse(claims.WsID)
		if err != nil {
			http.Error(w, "invalid workspace", http.StatusUnauthorized)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("ws upgrade failed")
		return
	}

	client := newClient(h, conn, wsID, userID)
	h.register <- client

	// Send connected message
	client.sendMsg(ServerMessage{
		Type: "connected",
		Payload: map[string]string{
			"workspace_id": wsID.String(),
			"user_id":      userID.String(),
		},
	})

	go client.writePump()
	go client.readPump()
}

// Close sends close frames to all clients.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		client.conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"))
		client.conn.Close()
		delete(h.clients, client)
	}
}
