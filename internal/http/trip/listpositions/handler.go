package listpositions

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
	points, err := h.uc.Execute(r.Context(), tripID)
	if err != nil {
		status, _ := domainerror.Problem(err)
		if status >= 500 {
			h.logger.ErrorContext(r.Context(), "list trip positions failed", slog.Any("error", err))
		}
		domainerror.Write(w, r, err)
		return
	}

	positions := make([]api.TripPosition, 0, len(points))
	for _, point := range points {
		positions = append(positions, position.ToAPI(point))
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(api.TripPositionsResponse{Positions: positions})
}
