package create

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/http/domainerror"
)

type Handler struct {
	uc usecase
}

func New(uc usecase) *Handler {
	return &Handler{uc: uc}
}

func (h *Handler) Handle(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	in, err := decodeAndValidateTripData(r, params.IdempotencyKey)
	if err != nil {
		domainerror.Write(w, r, err)
		return
	}

	trip, isReplay, err := h.uc.Execute(r.Context(), *in)
	if err != nil {
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
