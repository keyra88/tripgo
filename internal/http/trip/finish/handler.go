package finish

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/api"
	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/http/domainerror"
)

type Handler struct {
	uc usecase
}

func New(uc usecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	if tripID == uuid.Nil {
		domainerror.Write(w, r, fmt.Errorf("%w: invalid trip id", domain.ErrInvalidRequest))
		return
	}

	trip, err := h.uc.Execute(r.Context(), tripID)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	apiTrip := toAPITrip(trip)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(apiTrip)
}
