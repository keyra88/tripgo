//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package create_test
package create

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/model"
)

type (
	tripRepository interface {
		CreateTrip(ctx context.Context, trip *model.Trip) error
		GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
	}

	idempotencyRepository interface {
		LockIdempotencyKey(ctx context.Context, key uuid.UUID) error
		GetIdempotencyRecord(ctx context.Context, key uuid.UUID) (*model.IdempotencyRecord, error)
		SaveIdempotencyRecord(ctx context.Context, record *model.IdempotencyRecord) error
	}

	historyRepository interface {
		AddStatusHistory(ctx context.Context, tripID uuid.UUID, fromStatus *string, toStatus string, reason *string, changedAt time.Time) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	metricsRecorder interface {
		IncTripsCreated(ctx context.Context)
	}
)
