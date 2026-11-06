//go:build integration

package tests

import (
	"context"
	"net/http"
	"testing"

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
	t.Cleanup(func() {
		_ = cnt.Shutdown(context.Background())
	})

	return cnt.Server.Handler
}
