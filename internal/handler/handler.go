package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/service"
)

type Handler struct {
	api.Unimplemented
	service *service.TripService
	pool    *pgxpool.Pool
	queryTimeout time.Duration
}

func New(service *service.TripService, pool *pgxpool.Pool, queryTimeout time.Duration) *Handler {
	return &Handler{
		service: service,
		pool:    pool,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	req, err := decodeAndValidateTripData(r)
	if err != nil {
		handleError(w, r, err)
		return
	}

	trip, isReplay, err := h.service.CreateTrip(r.Context(), *req, params.IdempotencyKey)
	if err != nil {
		handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if isReplay {
		w.WriteHeader(http.StatusOK)
	} else {
		w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", trip.Id.String()))
		w.WriteHeader(http.StatusCreated)
	}
	_ = json.NewEncoder(w).Encode(trip)
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	if tripId == uuid.Nil {
		handleError(w, r, fmt.Errorf("%w: invalid trip id", service.ErrInvalidRequest))
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	trip, err := h.service.GetTrip(ctx, tripId)
	if err != nil {
		handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(trip)
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	if tripId == uuid.Nil {
		handleError(w, r, fmt.Errorf("%w: invalid trip id", service.ErrInvalidRequest))
		return
	}

	trip, err := h.service.FinishTrip(r.Context(), tripId)
	if err != nil {
		handleError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(trip)
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	pingCtx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	if err := h.pool.Ping(pingCtx); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}
