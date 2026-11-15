package finish_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
	"github.com/keyra88/tripgo/internal/usecase/trip/finish"
)

func TestFinishUsecase_Execute(t *testing.T) {
	tripID := uuid.New()
	now := time.Now().UTC()

	activeTrip := &model.Trip{
		ID:        tripID,
		UserID:    uuid.New(),
		DriverID:  uuid.New(),
		Price:     1200,
		Status:    model.StatusActive,
		StartedAt: now.Add(-15 * time.Minute),
	}

	completedTrip := &model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   uuid.New(),
		Price:      1200,
		Status:     model.StatusCompleted,
		StartedAt:  now.Add(-30 * time.Minute),
		FinishedAt: &now,
	}

	t.Run("successful finish", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)
		metrics := NewMockmetricsRecorder(ctrl)

		tripCopy := *activeTrip
		tripRepo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(&tripCopy, nil)

		txMgr.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		)

		tripRepo.EXPECT().UpdateTrip(gomock.Any(), gomock.Any()).Return(nil)
		histRepo.EXPECT().AddStatusHistory(gomock.Any(), tripID, gomock.Any(), string(model.StatusCompleted), nil, gomock.Any()).Return(nil)
		metrics.EXPECT().IncTripsCompleted(gomock.Any())

		uc := finish.New(tripRepo, histRepo, txMgr, metrics, nil)
		res, err := uc.Execute(context.Background(), tripID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.Status != model.StatusCompleted {
			t.Errorf("expected status %s, got %s", model.StatusCompleted, res.Status)
		}
		if res.FinishedAt == nil {
			t.Errorf("expected finished_at to be set")
		}
	})

	t.Run("finish already completed trip returns ErrTripCompleted", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		tripCopy := *completedTrip
		tripRepo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(&tripCopy, nil)

		uc := finish.New(tripRepo, histRepo, txMgr, nil, nil)
		_, err := uc.Execute(context.Background(), tripID)

		if !errors.Is(err, domain.ErrTripCompleted) {
			t.Fatalf("expected ErrTripCompleted, got: %v", err)
		}
	})

	t.Run("trip not found returns ErrTripNotFound", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		tripRepo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(nil, domain.ErrTripNotFound)

		uc := finish.New(tripRepo, histRepo, txMgr, nil, nil)
		_, err := uc.Execute(context.Background(), tripID)

		if !errors.Is(err, domain.ErrTripNotFound) {
			t.Fatalf("expected ErrTripNotFound, got: %v", err)
		}
	})

	t.Run("nil uuid returns ErrInvalidRequest", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		uc := finish.New(tripRepo, histRepo, txMgr, nil, nil)
		_, err := uc.Execute(context.Background(), uuid.Nil)

		if !errors.Is(err, domain.ErrInvalidRequest) {
			t.Fatalf("expected ErrInvalidRequest, got: %v", err)
		}
	})

	t.Run("transaction error propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		tripCopy := *activeTrip
		tripRepo.EXPECT().GetTripByID(gomock.Any(), tripID).Return(&tripCopy, nil)

		txErr := errors.New("tx failed")
		txMgr.EXPECT().Do(gomock.Any(), gomock.Any()).Return(txErr)

		uc := finish.New(tripRepo, histRepo, txMgr, nil, nil)
		_, err := uc.Execute(context.Background(), tripID)

		if !errors.Is(err, txErr) {
			t.Fatalf("expected txErr, got: %v", err)
		}
	})
}
