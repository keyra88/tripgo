package addposition

import (
	"context"
	"fmt"
	"time"

	"github.com/keyra88/tripgo/internal/model"
)

type Usecase struct {
	repo      positionRepository
	txManager txManager
	checks    positionChecks
	metrics   metricsRecorder
	now       func() time.Time
}

func New(
	repo positionRepository,
	txManager txManager,
	checks positionChecks,
	metrics metricsRecorder,
	now func() time.Time,
) *Usecase {
	return &Usecase{repo: repo, txManager: txManager, checks: checks, metrics: metrics, now: now}
}

func (uc *Usecase) Execute(ctx context.Context, input model.Position) (*model.Position, error) {
	if err := input.Validate(uc.now()); err != nil {
		return nil, err
	}

	var saved *model.Position
	err := uc.txManager.Do(ctx, func(txCtx context.Context) error {
		// Updating the active trip first serializes the write with trip completion.
		if err := uc.repo.UpdateLastPositionAt(txCtx, input.TripID, input.RecordedAt); err != nil {
			return err
		}
		var err error
		saved, err = uc.repo.InsertPosition(txCtx, input)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("save position: %w", err)
	}

	uc.checks.Cancel(input.TripID)
	uc.metrics.IncPositionsSaved(ctx)
	return saved, nil
}
