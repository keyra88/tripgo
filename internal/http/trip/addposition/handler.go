package addposition

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/http/domainerror"
	"github.com/keyra88/tripgo/internal/http/trip/position"
)

type Handler struct {
	uc     usecase
	logger *slog.Logger
}

func New(uc usecase, logger *slog.Logger) *Handler {
	return &Handler{uc: uc, logger: logger}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	input, err := decodePosition(r.Body, tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}
	saved, err := h.uc.Execute(r.Context(), input)
	if err != nil {
		status, _ := domainerror.Problem(err)
		if status >= 500 {
			h.logger.ErrorContext(r.Context(), "save trip position failed", slog.Any("error", err))
		}
		domainerror.Write(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(position.ToAPI(*saved))
}
