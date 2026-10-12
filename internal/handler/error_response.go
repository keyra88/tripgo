package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/service"
)

func writeProblem(w http.ResponseWriter, r *http.Request, status int, probType, title, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	path := r.URL.Path
	problem := api.Problem{
		Type:     probType,
		Title:    title,
		Status:   int32(status),
		Code:     code,
		Detail:   &detail,
		Instance: &path,
	}

	_ = json.NewEncoder(w).Encode(problem)
}

func handleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidRequest):
		writeProblem(
			w, r,
			http.StatusBadRequest,
			"https://tripgo.example/problems/invalid-request",
			"Invalid request",
			"invalid_request",
			err.Error(),
		)
	case errors.Is(err, service.ErrTripNotFound):
		writeProblem(
			w, r,
			http.StatusNotFound,
			"https://tripgo.example/problems/trip-not-found",
			"Trip not found",
			"trip_not_found",
			"Trip was not found",
		)
	case errors.Is(err, service.ErrDriverBusy):
		writeProblem(
			w, r,
			http.StatusConflict,
			"https://tripgo.example/problems/driver-busy",
			"Driver busy",
			"driver_busy",
			"Driver already has an active trip",
		)
	case errors.Is(err, service.ErrTripCompleted):
		writeProblem(
			w, r,
			http.StatusConflict,
			"https://tripgo.example/problems/trip-completed",
			"Trip completed",
			"trip_completed",
			"Operation is not allowed for a completed trip",
		)
	case errors.Is(err, service.ErrIdempotencyConflict):
		writeProblem(
			w, r,
			http.StatusConflict,
			"https://tripgo.example/problems/idempotency-conflict",
			"Idempotency conflict",
			"idempotency_conflict",
			"Idempotency-Key was already used with a different request body",
		)
	default:
		writeProblem(
			w, r,
			http.StatusInternalServerError,
			"https://tripgo.example/problems/internal-error",
			"Internal Server Error",
			"internal_error",
			"Internal server error",
		)
	}
}
