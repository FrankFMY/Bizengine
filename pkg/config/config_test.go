package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_Defaults(t *testing.T) {
	unsetEnv(t,
		"SERVER_HOST", "SERVER_PORT", "SERVER_ALLOWED_ORIGINS",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"REDIS_ADDR", "REDIS_PASSWORD", "REDIS_DB",
		"NATS_URL", "NATS_ENABLED",
		"SESSION_TTL", "SESSION_SEANCE_TTL", "SESSION_COOKIE_SECURE",
		"CENTRIFUGO_API_URL", "CENTRIFUGO_API_KEY",
		"S3_ENDPOINT", "S3_BUCKET", "S3_REGION", "S3_ACCESS_KEY", "S3_SECRET_KEY",
		"LOG_LEVEL", "ENV",
	)

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
	assert.Equal(t, 72*time.Hour, cfg.Session.SessionTTL)
	assert.Equal(t, 10*time.Minute, cfg.Session.SeanceTTL)
	assert.Equal(t, false, cfg.Session.CookieSecure)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "development", cfg.Env)
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

func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()

	original := make(map[string]string, len(keys))
	present := make(map[string]bool, len(keys))
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			original[key] = value
			present[key] = true
		}
		require.NoError(t, os.Unsetenv(key))
	}

	t.Cleanup(func() {
		for _, key := range keys {
			if present[key] {
				_ = os.Setenv(key, original[key])
				continue
			}
			_ = os.Unsetenv(key)
		}
	})
}
