package create_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
	"github.com/keyra88/tripgo/internal/usecase/trip/create"
)

func computeTestHash(in create.CreateTripInput) string {
	h := sha256.New()
	fmt.Fprintf(h, "%s:%s:%s:%s:%s:%s:%d",
		in.UserID.String(),
		in.DriverID.String(),
		strconv.FormatFloat(in.StartPoint.Latitude, 'g', -1, 64),
		strconv.FormatFloat(in.StartPoint.Longitude, 'g', -1, 64),
		strconv.FormatFloat(in.EndPoint.Latitude, 'g', -1, 64),
		strconv.FormatFloat(in.EndPoint.Longitude, 'g', -1, 64),
		in.Price,
	)
	return hex.EncodeToString(h.Sum(nil))
}

func TestCreateUsecase_Execute(t *testing.T) {
	userID := uuid.New()
	driverID := uuid.New()
	startPoint := model.Point{Latitude: 55.75, Longitude: 37.61}
	endPoint := model.Point{Latitude: 55.76, Longitude: 37.62}

	validInput := create.CreateTripInput{
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: startPoint,
		EndPoint:   endPoint,
		Price:      1000,
	}

	t.Run("success without idempotency key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		idempRepo := NewMockidempotencyRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)
		metrics := NewMockmetricsRecorder(ctrl)

		txMgr.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		)

		tripRepo.EXPECT().CreateTrip(gomock.Any(), gomock.Any()).Return(nil)
		histRepo.EXPECT().AddStatusHistory(gomock.Any(), gomock.Any(), nil, string(model.StatusActive), nil, gomock.Any()).Return(nil)
		metrics.EXPECT().IncTripsCreated(gomock.Any())

		uc := create.New(tripRepo, idempRepo, histRepo, txMgr, metrics, nil)
		trip, isReplay, err := uc.Execute(context.Background(), validInput)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isReplay {
			t.Errorf("expected isReplay = false, got true")
		}
		if trip.Status != model.StatusActive {
			t.Errorf("expected status %s, got %s", model.StatusActive, trip.Status)
		}
	})

	t.Run("driver already has active trip returns ErrDriverBusy", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		idempRepo := NewMockidempotencyRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		txMgr.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		)

		tripRepo.EXPECT().CreateTrip(gomock.Any(), gomock.Any()).Return(domain.ErrDriverBusy)

		uc := create.New(tripRepo, idempRepo, histRepo, txMgr, nil, nil)
		_, _, err := uc.Execute(context.Background(), validInput)

		if !errors.Is(err, domain.ErrDriverBusy) {
			t.Fatalf("expected ErrDriverBusy, got: %v", err)
		}
	})

	t.Run("invalid input fails validation before tx", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		idempRepo := NewMockidempotencyRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		uc := create.New(tripRepo, idempRepo, histRepo, txMgr, nil, nil)
		invalidInput := validInput
		invalidInput.Price = -50

		_, _, err := uc.Execute(context.Background(), invalidInput)
		if err == nil {
			t.Fatal("expected validation error, got nil")
		}
	})

	t.Run("idempotency replay returns existing trip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		idempRepo := NewMockidempotencyRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		idempKey := uuid.New()
		inputWithKey := validInput
		inputWithKey.IdempotencyKey = &idempKey

		existingTrip := &model.Trip{
			ID:        uuid.New(),
			UserID:    userID,
			DriverID:  driverID,
			Price:     1000,
			Status:    model.StatusActive,
			StartedAt: time.Now().UTC().Add(-time.Hour),
		}

		expectedHash := computeTestHash(inputWithKey)

		txMgr.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		)

		idempRepo.EXPECT().LockIdempotencyKey(gomock.Any(), idempKey).Return(nil)
		idempRepo.EXPECT().GetIdempotencyRecord(gomock.Any(), idempKey).Return(&model.IdempotencyRecord{
			Key:         idempKey,
			RequestHash: expectedHash,
			TripID:      existingTrip.ID,
			ExpiresAt:   time.Now().UTC().Add(time.Hour),
		}, nil)
		tripRepo.EXPECT().GetTripByID(gomock.Any(), existingTrip.ID).Return(existingTrip, nil)

		uc := create.New(tripRepo, idempRepo, histRepo, txMgr, nil, nil)
		resTrip, isReplay, err := uc.Execute(context.Background(), inputWithKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isReplay {
			t.Errorf("expected isReplay = true, got false")
		}
		if resTrip.ID != existingTrip.ID {
			t.Errorf("expected trip id %v, got %v", existingTrip.ID, resTrip.ID)
		}
	})

	t.Run("idempotency conflict with different hash", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		idempRepo := NewMockidempotencyRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		idempKey := uuid.New()
		inputWithKey := validInput
		inputWithKey.IdempotencyKey = &idempKey

		txMgr.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		)

		idempRepo.EXPECT().LockIdempotencyKey(gomock.Any(), idempKey).Return(nil)
		idempRepo.EXPECT().GetIdempotencyRecord(gomock.Any(), idempKey).Return(&model.IdempotencyRecord{
			Key:         idempKey,
			RequestHash: "different-hash",
			TripID:      uuid.New(),
			ExpiresAt:   time.Now().UTC().Add(time.Hour),
		}, nil)

		uc := create.New(tripRepo, idempRepo, histRepo, txMgr, nil, nil)
		_, _, err := uc.Execute(context.Background(), inputWithKey)
		if !errors.Is(err, domain.ErrIdempotencyConflict) {
			t.Fatalf("expected ErrIdempotencyConflict, got: %v", err)
		}
	})

	t.Run("repository error in transaction propagates", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		idempRepo := NewMockidempotencyRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)

		dbErr := errors.New("db connection failure")
		txMgr.EXPECT().Do(gomock.Any(), gomock.Any()).Return(dbErr)

		uc := create.New(tripRepo, idempRepo, histRepo, txMgr, nil, nil)
		_, _, err := uc.Execute(context.Background(), validInput)
		if !errors.Is(err, dbErr) {
			t.Fatalf("expected dbErr, got %v", err)
		}
	})

	t.Run("success with new idempotency key", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		tripRepo := NewMocktripRepository(ctrl)
		idempRepo := NewMockidempotencyRepository(ctrl)
		histRepo := NewMockhistoryRepository(ctrl)
		txMgr := NewMocktxManager(ctrl)
		metrics := NewMockmetricsRecorder(ctrl)

		idempKey := uuid.New()
		inputWithKey := validInput
		inputWithKey.IdempotencyKey = &idempKey

		txMgr.EXPECT().Do(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, fn func(context.Context) error) error {
				return fn(ctx)
			},
		)

		idempRepo.EXPECT().LockIdempotencyKey(gomock.Any(), idempKey).Return(nil)
		idempRepo.EXPECT().GetIdempotencyRecord(gomock.Any(), idempKey).Return(nil, nil)
		tripRepo.EXPECT().CreateTrip(gomock.Any(), gomock.Any()).Return(nil)
		histRepo.EXPECT().AddStatusHistory(gomock.Any(), gomock.Any(), gomock.Any(), "active", gomock.Any(), gomock.Any()).Return(nil)
		idempRepo.EXPECT().SaveIdempotencyRecord(gomock.Any(), gomock.Any()).Return(nil)
		metrics.EXPECT().IncTripsCreated(gomock.Any())

		uc := create.New(tripRepo, idempRepo, histRepo, txMgr, metrics, nil)
		resTrip, isReplay, err := uc.Execute(context.Background(), inputWithKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isReplay {
			t.Errorf("expected isReplay = false, got true")
		}
		if resTrip == nil || resTrip.UserID != validInput.UserID {
			t.Errorf("unexpected trip result: %+v", resTrip)
		}
	})
}
