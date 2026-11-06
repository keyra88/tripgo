package get_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
	"github.com/keyra88/tripgo/internal/usecase/trip/get"
)

func TestGetUsecase_Execute(t *testing.T) {
	tripID := uuid.New()
	validTrip := &model.Trip{
		ID:        tripID,
		UserID:    uuid.New(),
		DriverID:  uuid.New(),
		Price:     750,
		Status:    model.StatusActive,
		StartedAt: time.Now().UTC(),
	}

	t.Run("success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		repo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(validTrip, nil)

		uc := get.New(repo, nil)
		res, err := uc.Execute(context.Background(), tripID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID != tripID {
			t.Errorf("expected trip id %v, got %v", tripID, res.ID)
		}
	})

	t.Run("nil uuid returns ErrInvalidRequest", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		uc := get.New(repo, nil)

		_, err := uc.Execute(context.Background(), uuid.Nil)
		if !errors.Is(err, domain.ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest, got: %v", err)
		}
	})

	t.Run("trip not found returns ErrTripNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		repo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(nil, domain.ErrTripNotFound)

		uc := get.New(repo, nil)
		_, err := uc.Execute(context.Background(), tripID)

		if !errors.Is(err, domain.ErrTripNotFound) {
			t.Fatalf("expected ErrTripNotFound, got: %v", err)
		}
	})

	t.Run("repo error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		repo := NewMocktripRepository(ctrl)
		dbErr := errors.New("db error")
		repo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(nil, dbErr)

		uc := get.New(repo, nil)
		_, err := uc.Execute(context.Background(), tripID)

		if !errors.Is(err, dbErr) {
			t.Fatalf("expected dbErr, got: %v", err)
		}
	})
}
