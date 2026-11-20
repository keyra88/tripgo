//go:generate mockgen -source ${GOFILE} -destination mocks_test.go -package ${GOPACKAGE}
package positions

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/model"
)

type tripRepository interface {
	ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error)
}

type pushGateway interface {
	RequestPosition(ctx context.Context, tripID, driverID uuid.UUID) error
}

type ticker interface {
	Ticks() <-chan time.Time
	Stop()
}

type clockTicker struct {
	*time.Ticker
}

func (t clockTicker) Ticks() <-chan time.Time {
	return t.C
}
