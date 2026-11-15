package trip

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/http/trip/create"
	"github.com/keyra88/tripgo/internal/http/trip/finish"
	"github.com/keyra88/tripgo/internal/http/trip/get"
)

type Pinger interface {
	Ping(ctx context.Context) error
}

type Server struct {
	api.Unimplemented
	createHandler *create.Handler
	getHandler    *get.Handler
	finishHandler *finish.Handler
	pinger        Pinger
	queryTimeout  time.Duration
}

func NewServer(
	createHandler *create.Handler,
	getHandler *get.Handler,
	finishHandler *finish.Handler,
	pinger Pinger,
	queryTimeout time.Duration,
) *Server {
	return &Server{
		createHandler: createHandler,
		getHandler:    getHandler,
		finishHandler: finishHandler,
		pinger:        pinger,
		queryTimeout:  queryTimeout,
	}
}

func (s *Server) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	s.createHandler.Handle(w, r, params)
}

func (s *Server) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.getHandler.Handle(w, r, tripId)
}

func (s *Server) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	s.finishHandler.Handle(w, r, tripId)
}

func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	_ = r.Context()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}

func (s *Server) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.pinger != nil {
		ctx := r.Context()
		if s.queryTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.queryTimeout)
			defer cancel()
		}
		if err := s.pinger.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Unavailable})
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(api.HealthResponse{Status: api.Ok})
}
