// Package types provides shared domain types used across the application.
package types

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Entity represents a core business object in the ECS model.
type Entity struct {
	ID          uuid.UUID       `json:"id"`
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	Kind        string          `json:"kind"`
	Name        string          `json:"name"`
	Status      string          `json:"status"`
	ParentID    *uuid.UUID      `json:"parent_id,omitempty"`
	Meta        json.RawMessage `json:"meta"`
	SortOrder   int             `json:"sort_order"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	DeletedAt   *time.Time      `json:"deleted_at,omitempty"`

	// Loaded via include=components
	Components []Component `json:"components,omitempty"`
}

// Component represents a data packet attached to an entity.
type Component struct {
	ID          uuid.UUID       `json:"id"`
	EntityID    uuid.UUID       `json:"entity_id"`
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data"`
	Version     int64           `json:"version"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// Event represents something that happened in the system.
type Event struct {
	ID          uuid.UUID       `json:"id"`
	WorkspaceID uuid.UUID      `json:"workspace_id"`
	EntityID    *uuid.UUID      `json:"entity_id,omitempty"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data"`
	ActorID     *uuid.UUID      `json:"actor_id,omitempty"`
	Timestamp   time.Time       `json:"timestamp"`
	Version     int64           `json:"version"`
}

// User represents an authenticated user.
type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	FullName     string     `json:"full_name"`
	Phone        *string    `json:"phone,omitempty"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// Workspace represents a tenant.
type Workspace struct {
	ID        uuid.UUID       `json:"id"`
	Name      string          `json:"name"`
	Slug      string          `json:"slug"`
	OwnerID   uuid.UUID       `json:"owner_id"`
	Plan      string          `json:"plan"`
	Settings  json.RawMessage `json:"settings"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// WorkspaceMember represents a user's membership in a workspace.
type WorkspaceMember struct {
	WorkspaceID uuid.UUID       `json:"workspace_id"`
	UserID      uuid.UUID       `json:"user_id"`
	Role        string          `json:"role"`
	Permissions json.RawMessage `json:"permissions"`
	JoinedAt    time.Time       `json:"joined_at"`
}

// PageRequest holds pagination parameters.
type PageRequest struct {
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Sort   string `json:"sort"`
	Order  string `json:"order"`
}

// DefaultPageRequest returns pagination with default values.
func DefaultPageRequest() PageRequest {
	return PageRequest{
		Limit:  50,
		Offset: 0,
		Sort:   "created_at",
		Order:  "desc",
	}
}

// Normalize ensures pagination values are within valid bounds.
func (p *PageRequest) Normalize() {
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Limit > 200 {
		p.Limit = 200
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	if p.Sort == "" {
		p.Sort = "created_at"
	}
	if p.Order != "asc" {
		p.Order = "desc"
	}
}

// PageResponse is a generic paginated response.
type PageResponse[T any] struct {
	Items  []T `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Response is the standard API envelope.
type Response struct {
	OK         bool           `json:"ok"`
	Data       any            `json:"data,omitempty"`
	Error      *ResponseError `json:"error,omitempty"`
	Validation any            `json:"validation,omitempty"`
}

// ResponseError holds a machine-readable error code and optional details.
type ResponseError struct {
	Code    string `json:"code"`
	Details string `json:"details,omitempty"`
}

// ErrorResponse is the legacy API error format.
// Deprecated: use Response instead.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details"`
}
