//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/config"
	triphttp "github.com/keyra88/tripgo/internal/http/trip"
	"github.com/keyra88/tripgo/internal/infra/postgres"
)

func TestHealthAndReady(t *testing.T) {
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	srv := setupTestServer(t)

	reqHealth := httptest.NewRequest(http.MethodGet, "/health", nil)
	rrHealth := httptest.NewRecorder()
	srv.ServeHTTP(rrHealth, reqHealth)

	if rrHealth.Code != http.StatusOK {
		t.Errorf("expected 200 for /health, got %d", rrHealth.Code)
	}

	var healthResp api.HealthResponse
	if err := json.Unmarshal(rrHealth.Body.Bytes(), &healthResp); err != nil {
		t.Fatalf("failed to unmarshal health response: %v", err)
	}
	if healthResp.Status != api.Ok {
		t.Errorf("expected health status 'ok', got '%s'", healthResp.Status)
	}

	reqReady := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rrReady := httptest.NewRecorder()
	srv.ServeHTTP(rrReady, reqReady)

	if rrReady.Code != http.StatusOK {
		t.Errorf("expected 200 for /ready, got %d", rrReady.Code)
	}
	if ct := rrReady.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json' for /ready 200, got '%s'", ct)
	}

	var readyResp api.HealthResponse
	if err := json.Unmarshal(rrReady.Body.Bytes(), &readyResp); err != nil {
		t.Fatalf("failed to unmarshal ready response: %v", err)
	}
	if readyResp.Status != api.Ok {
		t.Errorf("expected ready status 'ok', got '%s'", readyResp.Status)
	}

	closedPoolCfg, _ := config.Load()
	closedPool, _ := postgres.NewPool(context.Background(), closedPoolCfg)
	closedPool.Close() 
	handlerUnavail := triphttp.NewServer(nil, nil, nil, closedPool, cfg.DBQueryTimeout)

	reqUnavail := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rrUnavail := httptest.NewRecorder()
	handlerUnavail.Ready(rrUnavail, reqUnavail)

	if rrUnavail.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 for unavailable DB, got %d", rrUnavail.Code)
	}
	if ct := rrUnavail.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json' for /ready 503, got '%s'", ct)
	}
	var unavailResp api.HealthResponse
	_ = json.Unmarshal(rrUnavail.Body.Bytes(), &unavailResp)
	if unavailResp.Status != api.Unavailable {
		t.Errorf("expected status 'unavailable', got '%s'", unavailResp.Status)
	}
}

func TestTripLifecycle(t *testing.T) {
	srv := setupTestServer(t)

	driverID := uuid.New()
	userID := uuid.New()

	tripReq := api.CreateTripJSONRequestBody{
		UserId:   userID,
		DriverId: driverID,
		StartPoint: api.Coordinates{
			Latitude:  59.9398,
			Longitude: 30.3146,
		},
		EndPoint: api.Coordinates{
			Latitude:  59.9290,
			Longitude: 30.3626,
		},
		Price: 1450,
	}

	bodyBytes, _ := json.Marshal(tripReq)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d, body: %s", rr.Code, rr.Body.String())
	}

	loc := rr.Header().Get("Location")
	if loc == "" {
		t.Errorf("expected Location header in 201 response")
	}

	var createdTrip api.Trip
	if err := json.Unmarshal(rr.Body.Bytes(), &createdTrip); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if createdTrip.Status != api.Active {
		t.Errorf("expected status 'active', got %s", createdTrip.Status)
	}
	if createdTrip.FinishedAt != nil {
		t.Errorf("expected finished_at to be nil for active trip")
	}
	if createdTrip.Price != 1450 {
		t.Errorf("expected price 1450, got %d", createdTrip.Price)
	}

	tripID := createdTrip.Id

	reqConflict := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
	reqConflict.Header.Set("Content-Type", "application/json")
	rrConflict := httptest.NewRecorder()
	srv.ServeHTTP(rrConflict, reqConflict)

	if rrConflict.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for busy driver, got %d, body: %s", rrConflict.Code, rrConflict.Body.String())
	}

	var conflictProb api.Problem
	if err := json.Unmarshal(rrConflict.Body.Bytes(), &conflictProb); err != nil {
		t.Fatalf("failed to decode problem json: %v", err)
	}
	if conflictProb.Code != "driver_busy" {
		t.Errorf("expected code 'driver_busy', got '%s'", conflictProb.Code)
	}

	reqGet := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/trips/%s", tripID), nil)
	rrGet := httptest.NewRecorder()
	srv.ServeHTTP(rrGet, reqGet)

	if rrGet.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for get trip, got %d", rrGet.Code)
	}

	var fetchedTrip api.Trip
	if err := json.Unmarshal(rrGet.Body.Bytes(), &fetchedTrip); err != nil {
		t.Fatalf("failed to decode fetched trip: %v", err)
	}
	if fetchedTrip.Id != tripID {
		t.Errorf("expected trip id %s, got %s", tripID, fetchedTrip.Id)
	}

	reqFinish := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/trips/%s/finish", tripID), nil)
	rrFinish := httptest.NewRecorder()
	srv.ServeHTTP(rrFinish, reqFinish)

	if rrFinish.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for finish trip, got %d, body: %s", rrFinish.Code, rrFinish.Body.String())
	}

	var finishedTrip api.Trip
	if err := json.Unmarshal(rrFinish.Body.Bytes(), &finishedTrip); err != nil {
		t.Fatalf("failed to decode finished trip: %v", err)
	}
	if finishedTrip.Status != api.Completed {
		t.Errorf("expected status 'completed', got %s", finishedTrip.Status)
	}
	if finishedTrip.FinishedAt == nil {
		t.Errorf("expected finished_at to be non-nil after finish")
	}

	reqFinishAgain := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/trips/%s/finish", tripID), nil)
	rrFinishAgain := httptest.NewRecorder()
	srv.ServeHTTP(rrFinishAgain, reqFinishAgain)

	if rrFinishAgain.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for already finished trip, got %d, body: %s", rrFinishAgain.Code, rrFinishAgain.Body.String())
	}

	var finishProb api.Problem
	if err := json.Unmarshal(rrFinishAgain.Body.Bytes(), &finishProb); err != nil {
		t.Fatalf("failed to decode problem json: %v", err)
	}
	if finishProb.Code != "trip_completed" {
		t.Errorf("expected code 'trip_completed', got '%s'", finishProb.Code)
	}

	reqNewTrip := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
	reqNewTrip.Header.Set("Content-Type", "application/json")
	rrNewTrip := httptest.NewRecorder()
	srv.ServeHTTP(rrNewTrip, reqNewTrip)

	if rrNewTrip.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for driver with completed trip, got %d, body: %s", rrNewTrip.Code, rrNewTrip.Body.String())
	}
}
