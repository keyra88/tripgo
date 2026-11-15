package create

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/keyra88/tripgo/api"
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

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	in, err := decodeAndValidateTripData(r, params.IdempotencyKey)
	if err != nil {
		h.logError(r.Context(), "decode and validate create trip request failed", err)
		domainerror.Write(w, r, err)
		return
	}

	trip, isReplay, err := h.uc.Execute(r.Context(), *in)
	if err != nil {
		h.logError(r.Context(), "create trip usecase failed", err)
		domainerror.Write(w, r, err)
		return
	}

	apiTrip := toAPITrip(trip)
	w.Header().Set("Content-Type", "application/json")
	if isReplay {
		w.WriteHeader(http.StatusOK)
	} else {
		w.Header().Set("Location", fmt.Sprintf("/api/v1/trips/%s", trip.ID.String()))
		w.WriteHeader(http.StatusCreated)
	}
	_ = json.NewEncoder(w).Encode(apiTrip)
}
