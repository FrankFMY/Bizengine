// Package redis provides Redis-backed storage implementations.
package redis

import (
	"github.com/redis/go-redis/v9"

	"github.com/bizengine/engine/pkg/config"
)

// NewClient creates a new Redis client from configuration.
func NewClient(cfg config.RedisConfig) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
}
