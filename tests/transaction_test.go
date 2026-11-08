//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/config"
	"github.com/keyra88/tripgo/internal/http/middleware"
	"github.com/keyra88/tripgo/internal/infra/postgres"
	"github.com/keyra88/tripgo/internal/infra/postgres/transaction"
	"github.com/keyra88/tripgo/internal/model"
	repotrip "github.com/keyra88/tripgo/internal/repository/trip"
)

func TestPanicRecovery(t *testing.T) {
	r := chi.NewRouter()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	r.Use(middleware.Recoverer(logger))
	r.Get("/panic-test", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic before response")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-test", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 Internal Server Error, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != contentTypeProblemJSON {
		t.Errorf("expected application/problem+json, got %s", ct)
	}
	var prob api.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem json: %v", err)
	}
	if prob.Code != "internal_error" {
		t.Errorf("expected code 'internal_error', got '%s'", prob.Code)
	}
	if prob.Status != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", prob.Status)
	}
}

func TestAtomicRollback(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	pool, err := postgres.NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	txMgr := transaction.New(pool, cfg.DBQueryTimeout)
	repo := repotrip.New(pool, cfg.DBQueryTimeout)

	testTripID := uuid.New()
	trip := &model.Trip{
		ID:         testTripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Point{Latitude: 59.9, Longitude: 30.3},
		EndPoint:   model.Point{Latitude: 59.8, Longitude: 30.2},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  time.Now().UTC(),
	}

	err = txMgr.Do(context.Background(), func(ctx context.Context) error {
		if err := repo.CreateTrip(ctx, trip); err != nil {
			return err
		}
		return fmt.Errorf("simulated failure after insert")
	})

	if err == nil {
		t.Fatalf("expected error from Do, got nil")
	}

	_, getErr := repo.GetTripByID(context.Background(), testTripID)
	if getErr == nil {
		t.Errorf("expected trip to not exist after rollback, but it was found")
	}

	panicTripID := uuid.New()
	panicTrip := &model.Trip{
		ID:         panicTripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		StartPoint: model.Point{Latitude: 59.9, Longitude: 30.3},
		EndPoint:   model.Point{Latitude: 59.8, Longitude: 30.2},
		Price:      500,
		Status:     model.StatusActive,
		StartedAt:  time.Now().UTC(),
	}

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("expected panic to be propagated")
			}
		}()

		_ = txMgr.Do(context.Background(), func(ctx context.Context) error {
			_ = repo.CreateTrip(ctx, panicTrip)
			panic("something crashed")
		})
	}()

	_, getErr2 := repo.GetTripByID(context.Background(), panicTripID)
	if getErr2 == nil {
		t.Errorf("expected trip to not exist after panic rollback, but it was found")
	}
}
