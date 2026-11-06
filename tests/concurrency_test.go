//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/api"
)

func TestConcurrencyDriverConflict(t *testing.T) {
	srv := setupTestServer(t)

	driverID := uuid.New()
	const count = 20

	var (
		createdCount  int32
		conflictCount int32
		wg            sync.WaitGroup
	)

	startBarrier := make(chan struct{})

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			tripReq := api.CreateTripJSONRequestBody{
				UserId:   uuid.New(),
				DriverId: driverID,
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
			body, _ := json.Marshal(tripReq)

			<-startBarrier 

			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			srv.ServeHTTP(rr, req)

			switch rr.Code {
			case http.StatusCreated:
				atomic.AddInt32(&createdCount, 1)
			case http.StatusConflict:
				atomic.AddInt32(&conflictCount, 1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	if createdCount != 1 {
		t.Errorf("expected exactly 1 created trip under race condition, got %d", createdCount)
	}
	if conflictCount != count-1 {
		t.Errorf("expected %d conflict responses (409), got %d", count-1, conflictCount)
	}
}

func TestConcurrencyFinish(t *testing.T) {
	srv := setupTestServer(t)

	tripReq := api.CreateTripJSONRequestBody{
		UserId:     uuid.New(),
		DriverId:   uuid.New(),
		StartPoint: api.Coordinates{Latitude: 59.9398, Longitude: 30.3146},
		EndPoint:   api.Coordinates{Latitude: 59.9290, Longitude: 30.3626},
		Price:      1200,
	}
	body, _ := json.Marshal(tripReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rr.Code)
	}

	var createdTrip api.Trip
	_ = json.Unmarshal(rr.Body.Bytes(), &createdTrip)
	tripID := createdTrip.Id

	const count = 10
	var (
		okCount       int32
		conflictCount int32
		wg            sync.WaitGroup
	)

	startBarrier := make(chan struct{})

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startBarrier

			reqFinish := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/trips/%s/finish", tripID), nil)
			rrFinish := httptest.NewRecorder()
			srv.ServeHTTP(rrFinish, reqFinish)

			switch rrFinish.Code {
			case http.StatusOK:
				atomic.AddInt32(&okCount, 1)
			case http.StatusConflict:
				atomic.AddInt32(&conflictCount, 1)
			}
		}()
	}

	close(startBarrier)
	wg.Wait()

	if okCount != 1 {
		t.Errorf("expected exactly 1 finish success (200), got %d", okCount)
	}
	if conflictCount != count-1 {
		t.Errorf("expected %d conflict responses (409), got %d", count-1, conflictCount)
	}
}
