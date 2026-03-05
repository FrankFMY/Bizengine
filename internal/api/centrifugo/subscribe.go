package centrifugo

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/bizengine/engine/internal/core/auth"
)

// SubscribeHandler handles Centrifugo subscribe proxy requests.
type SubscribeHandler struct {
	store     auth.SessionStore
	seanceTTL time.Duration
}

// NewSubscribeHandler creates a new subscribe proxy handler.
func NewSubscribeHandler(store auth.SessionStore, seanceTTL time.Duration) *SubscribeHandler {
	return &SubscribeHandler{store: store, seanceTTL: seanceTTL}
}

type subscribeRequest struct {
	Channel string `json:"channel"`
	User    string `json:"user"`
}

type subscribeResponse struct {
	Result *subscribeResult `json:"result,omitempty"`
	Error  *proxyError      `json:"error,omitempty"`
}

type subscribeResult struct{}

type proxyError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServeHTTP handles POST /internal/centrifugo/subscribe.
func (h *SubscribeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subscribeResponse{
			Error: &proxyError{Code: 400, Message: "invalid request"},
		})
		return
	}

	// Validate channel format: "org:{uuid}", "workspace:{uuid}", or "views:{seance_id}"
	switch {
	case strings.HasPrefix(req.Channel, "org:"):
		orgID := strings.TrimPrefix(req.Channel, "org:")
		if err := h.authorizeOrganization(r, orgID); err != nil {
			writeSubscribeError(w, err)
			return
		}

	case strings.HasPrefix(req.Channel, "workspace:"):
		orgID := strings.TrimPrefix(req.Channel, "workspace:")
		if err := h.authorizeOrganization(r, orgID); err != nil {
			writeSubscribeError(w, err)
			return
		}

	case strings.HasPrefix(req.Channel, "views:"):
		seanceID := strings.TrimPrefix(req.Channel, "views:")
		if err := h.authorizeSeance(r, seanceID); err != nil {
			writeSubscribeError(w, err)
			return
		}

	default:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(subscribeResponse{
			Error: &proxyError{Code: 403, Message: "invalid channel"},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subscribeResponse{
		Result: &subscribeResult{},
	})
}

func (h *SubscribeHandler) authorizeOrganization(r *http.Request, orgID string) *proxyError {
	sessionID := cookieValue(r, "teco_session")
	if sessionID == "" {
		return &proxyError{Code: 401, Message: "no session"}
	}
	sess, err := h.store.GetSession(r.Context(), sessionID)
	if err != nil {
		return &proxyError{Code: 401, Message: "session expired"}
	}
	if sess.OrganizationID.String() != orgID {
		return &proxyError{Code: 403, Message: "organization mismatch"}
	}
	return nil
}

func (h *SubscribeHandler) authorizeSeance(r *http.Request, seanceID string) *proxyError {
	cookieSeance := cookieValue(r, "teco_seance")
	if cookieSeance == "" {
		return &proxyError{Code: 401, Message: "no seance"}
	}
	if cookieSeance != seanceID {
		return &proxyError{Code: 403, Message: "seance mismatch"}
	}
	seance, err := h.store.GetSeance(r.Context(), seanceID)
	if err != nil {
		return &proxyError{Code: 401, Message: "seance expired"}
	}
	sessionID := cookieValue(r, "teco_session")
	if seance.SessionID != sessionID {
		return &proxyError{Code: 403, Message: "session mismatch"}
	}
	return nil
}

func writeSubscribeError(w http.ResponseWriter, pe *proxyError) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subscribeResponse{Error: pe})
}
