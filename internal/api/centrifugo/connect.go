package centrifugo

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/auth"
)

// ConversationIDsGetter retrieves user conversation IDs for real-time subscriptions.
type ConversationIDsGetter interface {
	GetUserConversationIDs(ctx context.Context, userID, orgID uuid.UUID) ([]uuid.UUID, error)
}

// ConnectHandler handles Centrifugo connect proxy requests.
type ConnectHandler struct {
	store      auth.SessionStore
	seanceTTL  time.Duration
	convGetter ConversationIDsGetter
}

// NewConnectHandler creates a new connect proxy handler.
func NewConnectHandler(store auth.SessionStore, seanceTTL time.Duration, convGetter ...ConversationIDsGetter) *ConnectHandler {
	h := &ConnectHandler{store: store, seanceTTL: seanceTTL}
	if len(convGetter) > 0 {
		h.convGetter = convGetter[0]
	}
	return h
}

type connectResult struct {
	User     string          `json:"user"`
	Channels []string        `json:"channels,omitempty"`
	Data     json.RawMessage `json:"data,omitempty"`
}

type connectResponse struct {
	Result *connectResult `json:"result,omitempty"`
	Error  *proxyError    `json:"error,omitempty"`
}

// ServeHTTP handles POST /api/internal/centrifugo/connect.
func (h *ConnectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := cookieValue(r, "teco_session")
	seanceID := cookieValue(r, "teco_seance")

	if sessionID == "" || seanceID == "" {
		writeConnectError(w, 403, "unauthorized")
		return
	}

	ctx := r.Context()

	seance, err := h.store.GetSeance(ctx, seanceID)
	if err != nil {
		writeConnectError(w, 403, "unauthorized")
		return
	}

	if seance.SessionID != sessionID {
		writeConnectError(w, 403, "unauthorized")
		return
	}

	sess, err := h.store.GetSession(ctx, sessionID)
	if err != nil {
		writeConnectError(w, 403, "unauthorized")
		return
	}

	h.store.SlideSeance(ctx, seanceID, h.seanceTTL)

	data, _ := json.Marshal(map[string]string{
		"user_id":         sess.UserID.String(),
		"organization_id": sess.OrganizationID.String(),
		"role":            sess.Role,
	})

	channels := []string{
		"org:" + sess.OrganizationID.String(),
		"workspace:" + sess.OrganizationID.String(),
		"views:" + seanceID,
	}

	if h.convGetter != nil {
		convIDs, err := h.convGetter.GetUserConversationIDs(ctx, sess.UserID, sess.OrganizationID)
		if err == nil {
			for _, cid := range convIDs {
				channels = append(channels, "chat:"+cid.String())
			}
		}
	}

	resp := connectResponse{
		Result: &connectResult{
			User:     sess.UserID.String(),
			Channels: channels,
			Data:     data,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func writeConnectError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(connectResponse{
		Error: &proxyError{Code: code, Message: message},
	})
}
