package get

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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
	logger       *slog.Logger
}

func New(uc usecase, queryTimeout time.Duration, logger *slog.Logger) *Handler {
	return &Handler{
		uc:           uc,
		queryTimeout: queryTimeout,
		logger:       logger,
	}
}

func (h *Handler) logError(ctx context.Context, msg string, err error) {
	if h.logger == nil {
		return
	}
	status, _ := domainerror.Problem(err)
	if status >= 500 {
		h.logger.ErrorContext(ctx, msg, slog.String("error", err.Error()))
	} else {
		h.logger.WarnContext(ctx, msg, slog.String("error", err.Error()))
	}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	if tripID == uuid.Nil {
		err := fmt.Errorf("%w: invalid trip id", domain.ErrInvalidRequest)
		h.logError(r.Context(), "invalid get trip request", err)
		domainerror.Write(w, r, err)
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
		h.logError(r.Context(), "get trip usecase failed", err)
		domainerror.Write(w, r, err)
		return
	}

	apiTrip := toAPITrip(trip)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiTrip)
}
