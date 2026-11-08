package finish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
)

type Usecase struct {
	tripRepo  tripRepository
	histRepo  historyRepository
	txManager txManager
	metrics   metricsRecorder
	logger    *slog.Logger
}

func New(
	tripRepo tripRepository,
	histRepo historyRepository,
	txManager txManager,
	metrics metricsRecorder,
	logger *slog.Logger,
) *Usecase {
	return &Usecase{
		tripRepo:  tripRepo,
		histRepo:  histRepo,
		txManager: txManager,
		metrics:   metrics,
		logger:    logger,
	}
}

func (uc *Usecase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid trip id", domain.ErrInvalidRequest)
	}

	trip, err := uc.tripRepo.GetTripByID(ctx, id)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if err := trip.Finish(now); err != nil {
		return nil, err
	}

	fromStatus := string(model.StatusActive)
	err = uc.txManager.Do(ctx, func(txCtx context.Context) error {
		if err := uc.tripRepo.UpdateTrip(txCtx, trip); err != nil {
			return err
		}
		if err := uc.histRepo.AddStatusHistory(txCtx, trip.ID, &fromStatus, string(model.StatusCompleted), nil, now); err != nil {
			return fmt.Errorf("add status history: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if uc.metrics != nil {
		uc.metrics.IncTripsCompleted(ctx)
	}
	if uc.logger != nil {
		uc.logger.InfoContext(ctx, "trip completed", slog.String("trip_id", trip.ID.String()))
	}

	return trip, nil
}
