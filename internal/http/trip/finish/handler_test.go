package finish_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/keyra88/tripgo/api"
	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/http/trip/finish"
	"github.com/keyra88/tripgo/internal/model"
)

func TestHandler_Handle(t *testing.T) {
	tripID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	now := time.Now().UTC()

	finishedTrip := &model.Trip{
		ID:         tripID,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Point{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Point{Latitude: 55.76, Longitude: 37.62},
		Price:      1000,
		Status:     model.StatusCompleted,
		StartedAt:  now.Add(-20 * time.Minute),
		FinishedAt: &now,
	}

	t.Run("success returns 200", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), tripID).Return(finishedTrip, nil)

		h := finish.New(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		rr := httptest.NewRecorder()

		h.Handle(rr, req, tripID)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
		var resp api.Trip
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Id != tripID {
			t.Errorf("expected trip id %v, got %v", tripID, resp.Id)
		}
		if resp.Status != api.Completed {
			t.Errorf("expected status %s, got %s", api.Completed, resp.Status)
		}
	})

	t.Run("already completed returns 409 trip_completed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), tripID).Return(nil, domain.ErrTripCompleted)

		h := finish.New(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		rr := httptest.NewRecorder()

		h.Handle(rr, req, tripID)

		if rr.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rr.Code)
		}
		var prob api.Problem
		_ = json.Unmarshal(rr.Body.Bytes(), &prob)
		if prob.Code != "trip_completed" {
			t.Errorf("expected code trip_completed, got %s", prob.Code)
		}
	})

	t.Run("not found returns 404 trip_not_found", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), tripID).Return(nil, domain.ErrTripNotFound)

		h := finish.New(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+tripID.String()+"/finish", nil)
		rr := httptest.NewRecorder()

		h.Handle(rr, req, tripID)

		if rr.Code != http.StatusNotFound {
			t.Fatalf("expected status 404, got %d", rr.Code)
		}
		var prob api.Problem
		_ = json.Unmarshal(rr.Body.Bytes(), &prob)
		if prob.Code != "trip_not_found" {
			t.Errorf("expected code trip_not_found, got %s", prob.Code)
		}
	})

	t.Run("nil uuid returns 400 invalid_request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		h := finish.New(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/00000000-0000-0000-0000-000000000000/finish", nil)
		rr := httptest.NewRecorder()

		h.Handle(rr, req, uuid.Nil)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rr.Code)
		}
		var prob api.Problem
		_ = json.Unmarshal(rr.Body.Bytes(), &prob)
		if prob.Code != "invalid_request" {
			t.Errorf("expected code invalid_request, got %s", prob.Code)
		}
	})
}
