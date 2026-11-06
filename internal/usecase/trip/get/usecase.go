package get

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
)

type Usecase struct {
	repo   tripRepository
	logger *slog.Logger
}

func New(repo tripRepository, logger *slog.Logger) *Usecase {
	return &Usecase{
		repo:   repo,
		logger: logger,
	}
}

func (uc *Usecase) Execute(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	if id == uuid.Nil {
		return nil, fmt.Errorf("%w: invalid trip id", domain.ErrInvalidRequest)
	}

	if uc.logger != nil {
		uc.logger.DebugContext(ctx, "getting trip by id", slog.String("trip_id", id.String()))
	}

	trip, err := uc.repo.GetTripByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return trip, nil
}
