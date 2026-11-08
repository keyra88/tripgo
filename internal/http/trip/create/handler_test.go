package create_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/keyra88/tripgo/api"
	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/http/trip/create"
	"github.com/keyra88/tripgo/internal/model"
)

func TestHandler_Handle(t *testing.T) {
	userID := uuid.New()
	driverID := uuid.New()
	tripID := uuid.New()

	validBody := map[string]any{
		"user_id":   userID.String(),
		"driver_id": driverID.String(),
		"start_point": map[string]any{
			"latitude":  55.75,
			"longitude": 37.61,
		},
		"end_point": map[string]any{
			"latitude":  55.76,
			"longitude": 37.62,
		},
		"price": 1000,
	}

	validCreatedTrip := &model.Trip{
		ID:         tripID,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Point{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Point{Latitude: 55.76, Longitude: 37.62},
		Price:      1000,
		Status:     model.StatusActive,
		StartedAt:  time.Now().UTC(),
	}

	t.Run("success created returns 201", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(validCreatedTrip, false, nil)

		h := create.New(uc)

		bodyBytes, _ := json.Marshal(validBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
		rr := httptest.NewRecorder()

		h.Handle(rr, req, api.CreateTripParams{})

		if rr.Code != http.StatusCreated {
			t.Fatalf("expected status 201, got %d", rr.Code)
		}
		loc := rr.Header().Get("Location")
		if loc != "/api/v1/trips/"+tripID.String() {
			t.Errorf("expected Location header /api/v1/trips/%s, got %s", tripID, loc)
		}
	})

	t.Run("idempotency replay returns 200", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(validCreatedTrip, true, nil)

		h := create.New(uc)

		bodyBytes, _ := json.Marshal(validBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
		rr := httptest.NewRecorder()

		idempKey := uuid.New()
		h.Handle(rr, req, api.CreateTripParams{IdempotencyKey: &idempKey})

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("invalid body returns 400 invalid_request", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		h := create.New(uc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader([]byte("{invalid-json")))
		rr := httptest.NewRecorder()

		h.Handle(rr, req, api.CreateTripParams{})

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rr.Code)
		}
		var prob api.Problem
		_ = json.Unmarshal(rr.Body.Bytes(), &prob)
		if prob.Code != "invalid_request" {
			t.Errorf("expected code invalid_request, got %s", prob.Code)
		}
	})

	t.Run("driver busy returns 409 driver_busy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, false, domain.ErrDriverBusy)

		h := create.New(uc)

		bodyBytes, _ := json.Marshal(validBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
		rr := httptest.NewRecorder()

		h.Handle(rr, req, api.CreateTripParams{})

		if rr.Code != http.StatusConflict {
			t.Fatalf("expected status 409, got %d", rr.Code)
		}
		var prob api.Problem
		_ = json.Unmarshal(rr.Body.Bytes(), &prob)
		if prob.Code != "driver_busy" {
			t.Errorf("expected code driver_busy, got %s", prob.Code)
		}
	})

	t.Run("repository error returns 500 internal_error with no details", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		uc := NewMockusecase(ctrl)
		uc.EXPECT().Execute(gomock.Any(), gomock.Any()).Return(nil, false, errors.New("secret sql db crash"))

		h := create.New(uc)

		bodyBytes, _ := json.Marshal(validBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", bytes.NewReader(bodyBytes))
		rr := httptest.NewRecorder()

		h.Handle(rr, req, api.CreateTripParams{})

		if rr.Code != http.StatusInternalServerError {
			t.Fatalf("expected status 500, got %d", rr.Code)
		}
		var prob api.Problem
		_ = json.Unmarshal(rr.Body.Bytes(), &prob)
		if prob.Code != "internal_error" {
			t.Errorf("expected code internal_error, got %s", prob.Code)
		}
		if prob.Detail != nil {
			t.Errorf("expected detail to be nil for 500, got %v", *prob.Detail)
		}
	})
}
