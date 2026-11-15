package finish

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/api"
	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/http/domainerror"
)

type Handler struct {
	uc     usecase
	logger *slog.Logger
}

func New(uc usecase, logger *slog.Logger) *Handler {
	return &Handler{uc: uc, logger: logger}
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
		h.logError(r.Context(), "invalid finish trip request", err)
		domainerror.Write(w, r, err)
		return
	}

	trip, err := h.uc.Execute(r.Context(), tripID)
	if err != nil {
		h.logError(r.Context(), "finish trip usecase failed", err)
		domainerror.Write(w, r, err)
		return
	}

	apiTrip := toAPITrip(trip)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiTrip)
}
