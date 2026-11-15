//go:build integration

package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/config"
)

func TestIdempotency(t *testing.T) {
	srv := setupTestServer(t)

	idempotencyKey := uuid.New().String()
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

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Idempotency-Key", idempotencyKey)
	rr1 := httptest.NewRecorder()
	srv.ServeHTTP(rr1, req1)

	if rr1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for first request with Idempotency-Key, got %d: %s", rr1.Code, rr1.Body.String())
	}

	var trip1 api.Trip
	if err := json.Unmarshal(rr1.Body.Bytes(), &trip1); err != nil {
		t.Fatalf("failed to decode trip response: %v", err)
	}

	// 2. Повторный запрос с тем же ключом и тем же телом -> 200 OK и та же поездка
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Idempotency-Key", idempotencyKey)
	rr2 := httptest.NewRecorder()
	srv.ServeHTTP(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for repeat request with same Idempotency-Key and body, got %d: %s", rr2.Code, rr2.Body.String())
	}

	var trip2 api.Trip
	if err := json.Unmarshal(rr2.Body.Bytes(), &trip2); err != nil {
		t.Fatalf("failed to decode repeat trip response: %v", err)
	}

	if trip1.Id != trip2.Id {
		t.Errorf("expected trip id %s, got %s", trip1.Id, trip2.Id)
	}

	diffTripReq := tripReq
	diffTripReq.Price = 9999
	diffBodyBytes, _ := json.Marshal(diffTripReq)

	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(diffBodyBytes))
	req3.Header.Set("Content-Type", "application/json")
	req3.Header.Set("Idempotency-Key", idempotencyKey)
	rr3 := httptest.NewRecorder()
	srv.ServeHTTP(rr3, req3)

	if rr3.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for different body, got %d: %s", rr3.Code, rr3.Body.String())
	}

	var prob3 api.Problem
	if err := json.Unmarshal(rr3.Body.Bytes(), &prob3); err != nil {
		t.Fatalf("failed to decode problem json: %v", err)
	}
	if prob3.Code != "idempotency_conflict" {
		t.Errorf("expected code 'idempotency_conflict', got '%s'", prob3.Code)
	}

	concurrentKey := uuid.New().String()
	concDriverID := uuid.New()
	concTripReq := api.CreateTripJSONRequestBody{
		UserId:   uuid.New(),
		DriverId: concDriverID,
		StartPoint: api.Coordinates{
			Latitude:  59.9398,
			Longitude: 30.3146,
		},
		EndPoint: api.Coordinates{
			Latitude:  59.9290,
			Longitude: 30.3626,
		},
		Price: 1000,
	}
	concBodyBytes, _ := json.Marshal(concTripReq)

	const concCount = 10
	var (
		createdCount int32
		replayCount  int32
		otherCount   int32
		wg           sync.WaitGroup
	)
	barrier := make(chan struct{})

	for i := 0; i < concCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-barrier

			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(concBodyBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", concurrentKey)
			rr := httptest.NewRecorder()
			srv.ServeHTTP(rr, req)

			switch rr.Code {
			case http.StatusCreated:
				atomic.AddInt32(&createdCount, 1)
			case http.StatusOK:
				atomic.AddInt32(&replayCount, 1)
			default:
				atomic.AddInt32(&otherCount, 1)
			}
		}()
	}

	close(barrier)
	wg.Wait()

	if createdCount != 1 {
		t.Errorf("expected exactly 1 created (201), got %d", createdCount)
	}
	if replayCount != concCount-1 {
		t.Errorf("expected %d replays (200), got %d (other=%d)", concCount-1, replayCount, otherCount)
	}

	precisionKey := uuid.New().String()
	pDriverID := uuid.New()
	pUserID := uuid.New()

	pReq1 := `{"user_id":"` + pUserID.String() + `","driver_id":"` + pDriverID.String() + `","start_point":{"latitude":59.9398,"longitude":30.3146},"end_point":{"latitude":59.9290,"longitude":30.3626},"price":1450}`
	r1 := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader([]byte(pReq1)))
	r1.Header.Set("Content-Type", "application/json")
	r1.Header.Set("Idempotency-Key", precisionKey)
	w1 := httptest.NewRecorder()
	srv.ServeHTTP(w1, r1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", w1.Code)
	}

	pReq2 := `{"user_id":"` + pUserID.String() + `","driver_id":"` + pDriverID.String() + `","start_point":{"latitude":59.9398001,"longitude":30.3146},"end_point":{"latitude":59.9290,"longitude":30.3626},"price":1450}`
	r2 := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader([]byte(pReq2)))
	r2.Header.Set("Content-Type", "application/json")
	r2.Header.Set("Idempotency-Key", precisionKey)
	w2 := httptest.NewRecorder()
	srv.ServeHTTP(w2, r2)
	if w2.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for 59.9398001 coordinate change, got %d, body: %s", w2.Code, w2.Body.String())
	}

	var trip1Precision api.Trip
	_ = json.Unmarshal(w1.Body.Bytes(), &trip1Precision)
	finishReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/trips/%s/finish", trip1Precision.Id), nil)
	finishRec := httptest.NewRecorder()
	srv.ServeHTTP(finishRec, finishReq)
	if finishRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for finishing trip, got %d: %s", finishRec.Code, finishRec.Body.String())
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config for db connection: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("failed to connect to db for expiring key: %v", err)
	}
	defer pool.Close()

	_, err = pool.Exec(context.Background(), "UPDATE idempotency_keys SET expires_at = now() - interval '1 hour' WHERE key = $1", precisionKey)
	if err != nil {
		t.Fatalf("failed to expire key in db: %v", err)
	}

	newDriverID := uuid.New()
	newUserID := uuid.New()
	expiredKeyReq := `{"user_id":"` + newUserID.String() + `","driver_id":"` + newDriverID.String() + `","start_point":{"latitude":59.9398,"longitude":30.3146},"end_point":{"latitude":59.9290,"longitude":30.3626},"price":1800}`
	rExpired := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader([]byte(expiredKeyReq)))
	rExpired.Header.Set("Content-Type", "application/json")
	rExpired.Header.Set("Idempotency-Key", precisionKey)
	wExpired := httptest.NewRecorder()
	srv.ServeHTTP(wExpired, rExpired)

	if wExpired.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created when reusing expired idempotency key, got %d, body: %s", wExpired.Code, wExpired.Body.String())
	}
}
