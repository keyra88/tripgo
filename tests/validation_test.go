//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/api"
)

func TestValidationErrors(t *testing.T) {
	srv := setupTestServer(t)

	nonExistentID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/trips/%s", nonExistentID), nil)
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent trip, got %d", rr.Code)
	}

	var prob api.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode problem: %v", err)
	}
	if prob.Code != "trip_not_found" {
		t.Errorf("expected code 'trip_not_found', got '%s'", prob.Code)
	}

	badReq := api.CreateTripJSONRequestBody{
		UserId:   uuid.New(),
		DriverId: uuid.New(),
		StartPoint: api.Coordinates{
			Latitude:  120.0,
			Longitude: 30.0,
		},
		EndPoint: api.Coordinates{
			Latitude:  50.0,
			Longitude: 30.0,
		},
		Price: 100,
	}
	body, _ := json.Marshal(badReq)
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(body))
	rrBad := httptest.NewRecorder()
	srv.ServeHTTP(rrBad, reqBad)

	if rrBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid latitude, got %d", rrBad.Code)
	}

	unknownFieldJSON := `{"user_id":"` + uuid.New().String() + `","driver_id":"` + uuid.New().String() + `","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.2},"price":100,"extra_field":"disallowed"}`
	reqUnknown := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader([]byte(unknownFieldJSON)))
	rrUnknown := httptest.NewRecorder()
	srv.ServeHTTP(rrUnknown, reqUnknown)

	if rrUnknown.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for unknown fields (additionalProperties: false), got %d", rrUnknown.Code)
	}

	reqInvalidUUID := httptest.NewRequest(http.MethodGet, "/api/v1/trips/123-not-a-uuid", nil)
	rrInvalidUUID := httptest.NewRecorder()
	srv.ServeHTTP(rrInvalidUUID, reqInvalidUUID)

	if rrInvalidUUID.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for non-UUID tripId, got %d", rrInvalidUUID.Code)
	}
	if ct := rrInvalidUUID.Header().Get("Content-Type"); ct != contentTypeProblemJSON {
		t.Errorf("expected application/problem+json, got %s", ct)
	}
	var invalidUUIDProb api.Problem
	if err := json.Unmarshal(rrInvalidUUID.Body.Bytes(), &invalidUUIDProb); err != nil {
		t.Fatalf("failed to decode problem json: %v", err)
	}
	if invalidUUIDProb.Code != "invalid_request" {
		t.Errorf("expected code 'invalid_request', got '%s'", invalidUUIDProb.Code)
	}

	testCases := []struct {
		name string
		json string
	}{
		{
			name: "missing price",
			json: `{"user_id":"` + uuid.New().String() + `","driver_id":"` + uuid.New().String() + `","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.2}}`,
		},
		{
			name: "case-insensitive PrIcE",
			json: `{"user_id":"` + uuid.New().String() + `","driver_id":"` + uuid.New().String() + `","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.2},"PrIcE":1450}`,
		},
		{
			name: "price null",
			json: `{"user_id":"` + uuid.New().String() + `","driver_id":"` + uuid.New().String() + `","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.2},"price":null}`,
		},
		{
			name: "start_point null",
			json: `{"user_id":"` + uuid.New().String() + `","driver_id":"` + uuid.New().String() + `","start_point":null,"end_point":{"latitude":59.8,"longitude":30.2},"price":100}`,
		},
		{
			name: "trailing tokens",
			json: `{"user_id":"` + uuid.New().String() + `","driver_id":"` + uuid.New().String() + `","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.2},"price":100} {}`,
		},
		{
			name: "float price",
			json: `{"user_id":"` + uuid.New().String() + `","driver_id":"` + uuid.New().String() + `","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.2},"price":100.5}`,
		},
		{
			name: "negative price",
			json: `{"user_id":"` + uuid.New().String() + `","driver_id":"` + uuid.New().String() + `","start_point":{"latitude":59.9,"longitude":30.3},"end_point":{"latitude":59.8,"longitude":30.2},"price":-10}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			reqTC := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader([]byte(tc.json)))
			reqTC.Header.Set("Content-Type", "application/json")
			rrTC := httptest.NewRecorder()
			srv.ServeHTTP(rrTC, reqTC)

			if rrTC.Code != http.StatusBadRequest {
				t.Errorf("[%s] expected 400 Bad Request, got %d, body: %s", tc.name, rrTC.Code, rrTC.Body.String())
			}
			if ct := rrTC.Header().Get("Content-Type"); ct != contentTypeProblemJSON {
				t.Errorf("[%s] expected application/problem+json, got %s", tc.name, ct)
			}
			var p api.Problem
			if err := json.Unmarshal(rrTC.Body.Bytes(), &p); err != nil {
				t.Fatalf("[%s] failed to decode problem: %v", tc.name, err)
			}
			if p.Code != "invalid_request" {
				t.Errorf("[%s] expected code 'invalid_request', got '%s'", tc.name, p.Code)
			}
		})
	}
}
