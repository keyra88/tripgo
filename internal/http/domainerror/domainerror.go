package domainerror

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keyra88/tripgo/api"
	domain "github.com/keyra88/tripgo/internal/domain/trip"
)

func Problem(err error) (int, api.Problem) {
	var valErr *domain.ValidationError
	switch {
	case errors.As(err, &valErr):
		detail := valErr.Error()
		return http.StatusBadRequest, api.Problem{
			Type:   "https://tripgo.example/problems/invalid-request",
			Title:  "Invalid request",
			Status: http.StatusBadRequest,
			Code:   "invalid_request",
			Detail: &detail,
		}
	case errors.Is(err, domain.ErrInvalidRequest):
		detail := err.Error()
		return http.StatusBadRequest, api.Problem{
			Type:   "https://tripgo.example/problems/invalid-request",
			Title:  "Invalid request",
			Status: http.StatusBadRequest,
			Code:   "invalid_request",
			Detail: &detail,
		}
	case errors.Is(err, domain.ErrTripNotFound):
		detail := "Trip was not found"
		return http.StatusNotFound, api.Problem{
			Type:   "https://tripgo.example/problems/trip-not-found",
			Title:  "Trip not found",
			Status: http.StatusNotFound,
			Code:   "trip_not_found",
			Detail: &detail,
		}
	case errors.Is(err, domain.ErrDriverBusy):
		detail := "Driver already has an active trip"
		return http.StatusConflict, api.Problem{
			Type:   "https://tripgo.example/problems/driver-busy",
			Title:  "Driver busy",
			Status: http.StatusConflict,
			Code:   "driver_busy",
			Detail: &detail,
		}
	case errors.Is(err, domain.ErrTripCompleted):
		detail := "Trip has already been completed"
		return http.StatusConflict, api.Problem{
			Type:   "https://tripgo.example/problems/trip-completed",
			Title:  "Trip already completed",
			Status: http.StatusConflict,
			Code:   "trip_completed",
			Detail: &detail,
		}
	case errors.Is(err, domain.ErrIdempotencyConflict):
		detail := "Idempotency key already used with different request parameters"
		return http.StatusConflict, api.Problem{
			Type:   "https://tripgo.example/problems/idempotency-conflict",
			Title:  "Idempotency conflict",
			Status: http.StatusConflict,
			Code:   "idempotency_conflict",
			Detail: &detail,
		}
	default:
		return http.StatusInternalServerError, api.Problem{
			Type:   "https://tripgo.example/problems/internal-error",
			Title:  "Internal server error",
			Status: http.StatusInternalServerError,
			Code:   "internal_error",
			Detail: nil,
		}
	}
}

func Write(w http.ResponseWriter, r *http.Request, err error) {
	status, prob := Problem(err)
	if r != nil && r.URL != nil {
		path := r.URL.Path
		prob.Instance = &path
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(prob)
}
