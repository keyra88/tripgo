package domainerror_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedCode   string
		checkNilDetail bool
	}{
		{
			name:           "validation error -> 400 invalid_request",
			err:            domain.NewValidationError("price", "must be positive"),
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_request",
		},
		{
			name:           "ErrInvalidRequest -> 400 invalid_request",
			err:            domain.ErrInvalidRequest,
			expectedStatus: http.StatusBadRequest,
			expectedCode:   "invalid_request",
		},
		{
			name:           "ErrTripNotFound -> 404 trip_not_found",
			err:            domain.ErrTripNotFound,
			expectedStatus: http.StatusNotFound,
			expectedCode:   "trip_not_found",
		},
		{
			name:           "ErrDriverBusy -> 409 driver_busy",
			err:            domain.ErrDriverBusy,
			expectedStatus: http.StatusConflict,
			expectedCode:   "driver_busy",
		},
		{
			name:           "ErrTripCompleted -> 409 trip_completed",
			err:            domain.ErrTripCompleted,
			expectedStatus: http.StatusConflict,
			expectedCode:   "trip_completed",
		},
		{
			name:           "ErrIdempotencyConflict -> 409 idempotency_conflict",
			err:            domain.ErrIdempotencyConflict,
			expectedStatus: http.StatusConflict,
			expectedCode:   "idempotency_conflict",
		},
		{
			name:           "unknown / repository error -> 500 internal_error with no details",
			err:            errors.New("pq: connection terminated unexpectedly"),
			expectedStatus: http.StatusInternalServerError,
			expectedCode:   "internal_error",
			checkNilDetail: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, body := domainerror.Problem(tc.err)
			if status != tc.expectedStatus {
				t.Errorf("status = %d, expected %d", status, tc.expectedStatus)
			}
			if body.Code != tc.expectedCode {
				t.Errorf("code = %q, expected %q", body.Code, tc.expectedCode)
			}
			if tc.checkNilDetail && body.Detail != nil {
				t.Errorf("expected Detail = nil for internal error, got %v", *body.Detail)
			}
		})
	}
}

func TestWrite(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/123", nil)
	rr := httptest.NewRecorder()

	domainerror.Write(rr, req, domain.ErrTripNotFound)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %q", ct)
	}
}
