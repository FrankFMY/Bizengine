package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/FrankFMY/arcana"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// HealthHandler provides extended health check.
type HealthHandler struct {
	pool         *pgxpool.Pool
	redis        *redis.Client
	arcanaEngine *arcana.Engine
	startedAt    time.Time
	version      string
}

// NewHealthHandler creates a HealthHandler.
func NewHealthHandler(pool *pgxpool.Pool, redis *redis.Client, arcanaEngine *arcana.Engine, version string) *HealthHandler {
	return &HealthHandler{
		pool:         pool,
		redis:        redis,
		arcanaEngine: arcanaEngine,
		startedAt:    time.Now(),
		version:      version,
	}
}

// Handle responds with detailed health information.
func (h *HealthHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()

	pgStatus := "ok"
	if err := h.pool.Ping(ctx); err != nil {
		pgStatus = "error"
	}

	redisStatus := "ok"
	if h.redis != nil {
		if err := h.redis.Ping(ctx).Err(); err != nil {
			redisStatus = "error"
		}
	} else {
		redisStatus = "not configured"
	}

	arcanaStatus := "stopped"
	if h.arcanaEngine != nil {
		stats := h.arcanaEngine.Stats()
		if stats.Running {
			arcanaStatus = "running"
		}
	}

	status := "ok"
	if pgStatus != "ok" || redisStatus == "error" {
		status = "degraded"
	}

	respondOK(w, http.StatusOK, map[string]any{
		"status":   status,
		"postgres": pgStatus,
		"redis":    redisStatus,
		"arcana":   arcanaStatus,
		"uptime":   time.Since(h.startedAt).String(),
		"version":  h.version,
	})
}
