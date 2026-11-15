//go:build integration

package tests

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/keyra88/tripgo/internal/config"
	"github.com/keyra88/tripgo/internal/container"
)

const contentTypeProblemJSON = "application/problem+json"

func setupTestServer(t *testing.T) http.Handler {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBConnectTimeout)
	defer cancel()

	cnt, err := container.New(ctx, cfg)
	if err != nil {
		t.Fatalf("failed to create container: %v", err)
	}

	cleanDB(t, cnt.Pool)
	t.Cleanup(func() {
		cleanDB(t, cnt.Pool)
		_ = cnt.Shutdown(context.Background())
	})

	return cnt.Server.Handler
}

func cleanDB(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := pool.Exec(ctx, "TRUNCATE TABLE trip_status_history, idempotency_keys, trips CASCADE;")
	if err != nil {
		t.Fatalf("failed to clean test database: %v", err)
	}
}
