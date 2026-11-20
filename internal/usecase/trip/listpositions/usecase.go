package listpositions

import (
	"context"

	"github.com/google/uuid"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
)

type Usecase struct {
	repo positionRepository
}

func New(repo positionRepository) *Usecase {
	return &Usecase{repo: repo}
}

func (uc *Usecase) Execute(ctx context.Context, id uuid.UUID) ([]model.Position, error) {
	if id == uuid.Nil {
		return nil, domain.NewValidationError("trip_id", "cannot be nil")
	}
	if _, err := uc.repo.GetTripByID(ctx, id); err != nil {
		return nil, err
	}
	return uc.repo.ListPositions(ctx, id)
}
