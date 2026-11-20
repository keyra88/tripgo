//go:build integration

package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/keyra88/tripgo/api"
)

func TestTripPositions(t *testing.T) {
	srv := setupTestServer(t)

	create, err := json.Marshal(api.CreateTripJSONRequestBody{UserId: uuid.New(), DriverId: uuid.New()})
	require.NoError(t, err)
	rr := positionRequest(srv, http.MethodPost, "/api/v1/trips", create)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	tripPath := rr.Header().Get("Location")
	path := tripPath + "/positions"

	rr = positionRequest(srv, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	require.JSONEq(t, `{"positions":[]}`, rr.Body.String())

	now := time.Now().UTC().Truncate(time.Microsecond)
	old := now.Add(-time.Minute)
	offset := time.FixedZone("UTC+05:30", int((5*time.Hour+30*time.Minute)/time.Second))
	first := savePosition(t, srv, path, now.In(offset))
	second := savePosition(t, srv, path, old)
	third := savePosition(t, srv, path, old)
	require.True(t, now.Equal(first.RecordedAt))
	require.Equal(t, api.Coordinates{Latitude: 59.93, Longitude: 30.33}, first.Coordinates)
	require.Equal(t, strings.TrimPrefix(tripPath, "/api/v1/trips/"), first.TripId.String())

	rr = positionRequest(srv, http.MethodGet, tripPath, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var trip api.Trip
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &trip))
	require.NotNil(t, trip.LastPositionAt)
	require.True(t, now.Equal(*trip.LastPositionAt))

	rr = positionRequest(srv, http.MethodPost, tripPath+"/finish", nil)
	require.Equal(t, http.StatusOK, rr.Code)
	rr = positionRequest(srv, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rr.Code)
	var route api.TripPositionsResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &route))
	require.Equal(t, []api.TripPosition{second, third, first}, route.Positions)

	assertPositionProblem(t, positionRequest(srv, http.MethodPost, path, positionBody(t, now)), http.StatusConflict, "trip_completed")
	missing := "/api/v1/trips/" + uuid.NewString() + "/positions"
	assertPositionProblem(t, positionRequest(srv, http.MethodGet, missing, nil), http.StatusNotFound, "trip_not_found")
	assertPositionProblem(t, positionRequest(srv, http.MethodPost, missing, positionBody(t, now)), http.StatusNotFound, "trip_not_found")
}

func TestTripPositionValidation(t *testing.T) {
	srv := setupTestServer(t)
	path := "/api/v1/trips/" + uuid.NewString() + "/positions"

	cases := map[string]string{
		"missing coordinates":        `{"recorded_at":"2026-08-24T12:00:05Z"}`,
		"missing latitude":           `{"coordinates":{"longitude":0},"recorded_at":"2026-08-24T12:00:05Z"}`,
		"null longitude":             `{"coordinates":{"latitude":0,"longitude":null},"recorded_at":"2026-08-24T12:00:05Z"}`,
		"latitude range":             `{"coordinates":{"latitude":91,"longitude":0},"recorded_at":"2026-08-24T12:00:05Z"}`,
		"longitude range":            `{"coordinates":{"latitude":0,"longitude":-181},"recorded_at":"2026-08-24T12:00:05Z"}`,
		"bad timestamp":              `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"invalid"}`,
		"offset hour out of range":   `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-08-24T12:00:05+24:00"}`,
		"offset minute out of range": `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-08-24T12:00:05+00:60"}`,
		"one-digit hour":             `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-08-24T1:00:05Z"}`,
		"comma fraction separator":   `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-08-24T12:00:05,12Z"}`,
		"null timestamp":             `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":null}`,
		"unknown field":              `{"coordinates":{"latitude":0,"longitude":0,"name":"extra"},"recorded_at":"2026-08-24T12:00:05Z"}`,
		"trailing data":              `{"coordinates":{"latitude":0,"longitude":0},"recorded_at":"2026-08-24T12:00:05Z"}{}`,
		"future":                     string(positionBody(t, time.Now().Add(2*time.Minute))),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			assertPositionProblem(t, positionRequest(srv, http.MethodPost, path, []byte(body)), http.StatusBadRequest, "invalid_request")
		})
	}
	assertPositionProblem(t, positionRequest(srv, http.MethodGet, "/api/v1/trips/invalid/positions", nil), http.StatusBadRequest, "invalid_request")
}

func positionRequest(srv http.Handler, method, path string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	return rr
}

func positionBody(t *testing.T, at time.Time) []byte {
	t.Helper()
	body, err := json.Marshal(api.PositionData{Coordinates: api.Coordinates{Latitude: 59.93, Longitude: 30.33}, RecordedAt: at})
	require.NoError(t, err)
	return body
}

func savePosition(t *testing.T, srv http.Handler, path string, at time.Time) api.TripPosition {
	t.Helper()
	rr := positionRequest(srv, http.MethodPost, path, positionBody(t, at))
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	var point api.TripPosition
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &point))
	return point
}

func assertPositionProblem(t *testing.T, rr *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	require.Equal(t, status, rr.Code, rr.Body.String())
	require.Equal(t, contentTypeProblemJSON, rr.Header().Get("Content-Type"))
	var problem api.Problem
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
	require.Equal(t, code, problem.Code)
}
