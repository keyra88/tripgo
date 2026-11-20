//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}_test
package addposition

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/model"
)

type positionRepository interface {
	UpdateLastPositionAt(ctx context.Context, id uuid.UUID, recordedAt time.Time) error
	InsertPosition(ctx context.Context, position model.Position) (*model.Position, error)
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type positionChecks interface {
	Cancel(id uuid.UUID)
}

type metricsRecorder interface {
	IncPositionsSaved(ctx context.Context)
}
