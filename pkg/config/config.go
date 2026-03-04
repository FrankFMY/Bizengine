// Package config provides application configuration loaded from environment variables.
package config

import (
	"context"
	"time"

	"github.com/sethvargo/go-envconfig"
)

// Config holds all application configuration.
type Config struct {
	Server   ServerConfig   `env:", prefix=SERVER_"`
	DB       DBConfig       `env:", prefix=DB_"`
	Redis    RedisConfig    `env:", prefix=REDIS_"`
	NATS     NATSConfig     `env:", prefix=NATS_"`
	JWT      JWTConfig      `env:", prefix=JWT_"`
	LogLevel string         `env:"LOG_LEVEL, default=info"`
	Env      string         `env:"ENV, default=development"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	Host string `env:"HOST, default=0.0.0.0"`
	Port int    `env:"PORT, default=8080"`
}

// DBConfig holds PostgreSQL connection settings.
type DBConfig struct {
	Host     string `env:"HOST, default=localhost"`
	Port     int    `env:"PORT, default=5432"`
	User     string `env:"USER, default=bizengine"`
	Password string `env:"PASSWORD, default=bizengine"`
	Name     string `env:"NAME, default=bizengine"`
	SSLMode  string `env:"SSLMODE, default=disable"`
}

// DSN returns the PostgreSQL connection string.
func (c DBConfig) DSN() string {
	return "postgres://" + c.User + ":" + c.Password + "@" + c.Host + ":" + itoa(c.Port) + "/" + c.Name + "?sslmode=" + c.SSLMode
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	Addr     string `env:"ADDR, default=localhost:6379"`
	Password string `env:"PASSWORD, default="`
	DB       int    `env:"DB, default=0"`
}

// NATSConfig holds NATS connection settings.
type NATSConfig struct {
	URL     string `env:"URL, default=nats://localhost:4222"`
	Enabled bool   `env:"ENABLED, default=false"`
}

// JWTConfig holds JWT authentication settings.
type JWTConfig struct {
	Secret     string        `env:"SECRET, required"`
	AccessTTL  time.Duration `env:"ACCESS_TTL, default=15m"`
	RefreshTTL time.Duration `env:"REFRESH_TTL, default=720h"`
}

// Load reads configuration from environment variables.
func Load(ctx context.Context) (*Config, error) {
	var cfg Config
	if err := envconfig.Process(ctx, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
