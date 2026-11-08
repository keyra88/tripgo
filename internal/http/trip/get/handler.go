package get

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/api"
	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/http/domainerror"
)

type Handler struct {
	uc           usecase
	queryTimeout time.Duration
}

func New(uc usecase, queryTimeout time.Duration) *Handler {
	return &Handler{
		uc:           uc,
		queryTimeout: queryTimeout,
	}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	if tripID == uuid.Nil {
		domainerror.Write(w, r, fmt.Errorf("%w: invalid trip id", domain.ErrInvalidRequest))
		return
	}

	ctx := r.Context()
	if h.queryTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.queryTimeout)
		defer cancel()
	}

	trip, err := h.uc.Execute(ctx, tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	apiTrip := toAPITrip(trip)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiTrip)
}
