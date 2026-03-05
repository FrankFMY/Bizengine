//go:build integration

// Package testutil provides test helpers for integration tests with PostgreSQL.
package testutil

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewTestPool starts a PostgreSQL container, runs migrations, and returns a pool.
func NewTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("bizengine_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	t.Cleanup(func() {
		pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Fatalf("failed to create pool: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
	})

	runMigrations(t, ctx, pool)
	return pool
}

func runMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	migrationsDir := findMigrationsDir(t)
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations dir: %v", err)
	}

	var upFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".sql" {
			if len(entry.Name()) > 7 && entry.Name()[len(entry.Name())-7:] == ".up.sql" {
				upFiles = append(upFiles, entry.Name())
			}
		}
	}
	sort.Strings(upFiles)

	for _, file := range upFiles {
		sql, err := os.ReadFile(filepath.Join(migrationsDir, file))
		if err != nil {
			t.Fatalf("failed to read migration %s: %v", file, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			t.Fatalf("failed to run migration %s: %v", file, err)
		}
	}
}

func findMigrationsDir(t *testing.T) string {
	t.Helper()
	// Walk up from cwd to find migrations/
	dir, _ := os.Getwd()
	for i := 0; i < 10; i++ {
		candidate := filepath.Join(dir, "migrations")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("migrations directory not found")
	return ""
}

// SeedOrganization creates a minimal organization + user for testing.
func SeedOrganization(t *testing.T, pool *pgxpool.Pool) (userID, orgID fmt.Stringer) {
	t.Helper()
	ctx := context.Background()

	var uid, wid string

	err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, full_name) VALUES ('test@example.com', '$2a$10$abcdefghijklmnopqrstuuABCDEFGHIJKLMNOPQRSTUVWXYZ0123456', 'Test') RETURNING id`,
	).Scan(&uid)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, owner_id) VALUES ('Test WS', 'test-ws', $1) RETURNING id`, uid,
	).Scan(&wid)
	if err != nil {
		t.Fatalf("seed org: %v", err)
	}

	return stringer(uid), stringer(wid)
}

type stringer string

func (s stringer) String() string { return string(s) }
