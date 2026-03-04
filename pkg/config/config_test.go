package config

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret-at-least-32-characters")

	cfg, err := Load(context.Background())
	require.NoError(t, err)

	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, "localhost", cfg.DB.Host)
	assert.Equal(t, 5432, cfg.DB.Port)
	assert.Equal(t, "bizengine", cfg.DB.User)
	assert.Equal(t, "disable", cfg.DB.SSLMode)
	assert.Equal(t, "localhost:6379", cfg.Redis.Addr)
	assert.Equal(t, false, cfg.NATS.Enabled)
	assert.Equal(t, 15*time.Minute, cfg.JWT.AccessTTL)
	assert.Equal(t, 720*time.Hour, cfg.JWT.RefreshTTL)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Env)
}

func TestLoad_MissingJWTSecret_Error(t *testing.T) {
	_, err := Load(context.Background())
	require.Error(t, err)
}

func TestDBConfig_DSN(t *testing.T) {
	cfg := DBConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user",
		Password: "pass",
		Name:     "db",
		SSLMode:  "disable",
	}
	assert.Equal(t, "postgres://user:pass@localhost:5432/db?sslmode=disable", cfg.DSN())
}
