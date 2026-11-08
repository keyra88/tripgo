//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package finish_test
package finish

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/model"
)

type (
	tripRepository interface {
		GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error)
		UpdateTrip(ctx context.Context, trip *model.Trip) error
	}

	historyRepository interface {
		AddStatusHistory(ctx context.Context, tripID uuid.UUID, fromStatus *string, toStatus string, reason *string, changedAt time.Time) error
	}

	txManager interface {
		Do(ctx context.Context, fn func(ctx context.Context) error) error
	}

	metricsRecorder interface {
		IncTripsCompleted(ctx context.Context)
	}
)
