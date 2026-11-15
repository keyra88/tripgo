package create

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
)

type CreateTripInput struct {
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartPoint     model.Point
	EndPoint       model.Point
	Price          int64
	IdempotencyKey *uuid.UUID
}

type Usecase struct {
	tripRepo  tripRepository
	idempRepo idempotencyRepository
	histRepo  historyRepository
	txManager txManager
	metrics   metricsRecorder
	logger    *slog.Logger
}

func New(
	tripRepo tripRepository,
	idempRepo idempotencyRepository,
	histRepo historyRepository,
	txManager txManager,
	metrics metricsRecorder,
	logger *slog.Logger,
) *Usecase {
	return &Usecase{
		tripRepo:  tripRepo,
		idempRepo: idempRepo,
		histRepo:  histRepo,
		txManager: txManager,
		metrics:   metrics,
		logger:    logger,
	}
}

func computeRequestHash(in CreateTripInput) string {
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

func (uc *Usecase) Execute(ctx context.Context, in CreateTripInput) (*model.Trip, bool, error) {
	now := time.Now().UTC()
	trip := &model.Trip{
		ID:         uuid.New(),
		UserID:     in.UserID,
		DriverID:   in.DriverID,
		StartPoint: in.StartPoint,
		EndPoint:   in.EndPoint,
		Price:      in.Price,
		Status:     model.StatusActive,
		StartedAt:  now,
		FinishedAt: nil,
	}

	if err := trip.Validate(); err != nil {
		return nil, false, err
	}

	if in.IdempotencyKey == nil {
		err := uc.txManager.Do(ctx, func(txCtx context.Context) error {
			if err := uc.tripRepo.CreateTrip(txCtx, trip); err != nil {
				return err
			}
			if err := uc.histRepo.AddStatusHistory(txCtx, trip.ID, nil, string(model.StatusActive), nil, now); err != nil {
				return fmt.Errorf("add status history: %w", err)
			}
			return nil
		})
		if err != nil {
			return nil, false, err
		}

		if uc.metrics != nil {
			uc.metrics.IncTripsCreated(ctx)
		}
		if uc.logger != nil {
			uc.logger.InfoContext(ctx, "trip created", slog.String("trip_id", trip.ID.String()))
		}
		return trip, false, nil
	}

	reqHash := computeRequestHash(in)
	var (
		resTrip  *model.Trip
		isReplay bool
	)

	err := uc.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := uc.idempRepo.LockIdempotencyKey(txCtx, *in.IdempotencyKey); err != nil {
			return fmt.Errorf("lock idempotency key: %w", err)
		}

		rec, err := uc.idempRepo.GetIdempotencyRecord(txCtx, *in.IdempotencyKey)
		if err != nil {
			return fmt.Errorf("get idempotency record: %w", err)
		}

		if rec != nil && rec.ExpiresAt.After(now) {
			if rec.RequestHash == reqHash {
				existingTrip, err := uc.tripRepo.GetTripByID(txCtx, rec.TripID)
				if err != nil {
					return fmt.Errorf("get existing trip by idempotency: %w", err)
				}
				resTrip = existingTrip
				isReplay = true
				return nil
			}
			return domain.ErrIdempotencyConflict
		}

		if err := uc.tripRepo.CreateTrip(txCtx, trip); err != nil {
			return err
		}

		if err := uc.histRepo.AddStatusHistory(txCtx, trip.ID, nil, string(model.StatusActive), nil, now); err != nil {
			return fmt.Errorf("add status history: %w", err)
		}

		newRec := &model.IdempotencyRecord{
			Key:         *in.IdempotencyKey,
			RequestHash: reqHash,
			TripID:      trip.ID,
			CreatedAt:   now,
			ExpiresAt:   now.Add(24 * time.Hour),
		}
		if err := uc.idempRepo.SaveIdempotencyRecord(txCtx, newRec); err != nil {
			return fmt.Errorf("save idempotency record: %w", err)
		}

		resTrip = trip
		isReplay = false
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	if !isReplay {
		if uc.metrics != nil {
			uc.metrics.IncTripsCreated(ctx)
		}
		if uc.logger != nil {
			uc.logger.InfoContext(ctx, "trip created", slog.String("trip_id", resTrip.ID.String()))
		}
	}

	return resTrip, isReplay, nil
}
