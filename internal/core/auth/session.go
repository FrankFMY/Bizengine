package auth

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Session represents an authenticated user session stored in Redis.
type Session struct {
	ID          string    `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	PhoneID     uuid.UUID `json:"phone_id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Role        string    `json:"role"`
	Email       string    `json:"email"`
	FullName    string    `json:"full_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// Seance represents an activity proof tied to a session.
type Seance struct {
	ID        string    `json:"id"`
	SessionID string    `json:"session_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SessionStore defines operations for session/seance persistence.
type SessionStore interface {
	CreateSession(ctx context.Context, s *Session, ttl time.Duration) error
	GetSession(ctx context.Context, id string) (*Session, error)
	UpdateSession(ctx context.Context, s *Session, ttl time.Duration) error
	DeleteSession(ctx context.Context, id string) error

	CreateSeance(ctx context.Context, s *Seance, ttl time.Duration) error
	GetSeance(ctx context.Context, id string) (*Seance, error)
	SlideSeance(ctx context.Context, id string, ttl time.Duration) error
	DeleteSessionSeances(ctx context.Context, sessionID string) error
}
