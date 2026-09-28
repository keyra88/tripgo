package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/service"
)

type Handler struct {
	api.Unimplemented
	service *service.TripService
	pool    *pgxpool.Pool
}

func New(service *service.TripService, pool *pgxpool.Pool) *Handler {
	return &Handler{
		service: service,
		pool:    pool,
	}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var req api.CreateTripJSONRequestBody
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		handleError(w, r, fmt.Errorf("%w: invalid json body or unknown fields: %v", service.ErrInvalidRequest, err))
		return
	}

	if req.UserId == uuid.Nil {
		handleError(w, r, fmt.Errorf("%w: user_id must be a valid non-empty uuid", service.ErrInvalidRequest))
		return
	}
	if req.DriverId == uuid.Nil {
		handleError(w, r, fmt.Errorf("%w: driver_id must be a valid non-empty uuid", service.ErrInvalidRequest))
		return
	}
	if req.StartPoint.Latitude < -90 || req.StartPoint.Latitude > 90 {
		handleError(w, r, fmt.Errorf("%w: start_point.latitude out of range [-90, 90]", service.ErrInvalidRequest))
		return
	}
	if req.StartPoint.Longitude < -180 || req.StartPoint.Longitude > 180 {
		handleError(w, r, fmt.Errorf("%w: start_point.longitude out of range [-180, 180]", service.ErrInvalidRequest))
		return
	}
	if req.EndPoint.Latitude < -90 || req.EndPoint.Latitude > 90 {
		handleError(w, r, fmt.Errorf("%w: end_point.latitude out of range [-90, 90]", service.ErrInvalidRequest))
		return
	}
	if req.EndPoint.Longitude < -180 || req.EndPoint.Longitude > 180 {
		handleError(w, r, fmt.Errorf("%w: end_point.longitude out of range [-180, 180]", service.ErrInvalidRequest))
		return
	}
	if req.Price < 0 {
		handleError(w, r, fmt.Errorf("%w: price must be >= 0", service.ErrInvalidRequest))
		return
	}

	trip, isReplay, err := h.service.CreateTrip(r.Context(), req, params.IdempotencyKey)
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

	trip, err := h.service.GetTrip(r.Context(), tripId)
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
	w.Header().Set("Content-Type", "application/json")
	if err := h.pool.Ping(r.Context()); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}
