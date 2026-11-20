package trip

import (
	"time"

	"github.com/google/uuid"
)

type Position struct {
	ID          int64
	TripID      uuid.UUID
	Coordinates Point
	RecordedAt  time.Time
}

func (p Position) Validate(now time.Time) error {
	if p.TripID == uuid.Nil {
		return NewValidationError("trip_id", "cannot be nil")
	}
	if p.RecordedAt.After(now.Add(time.Minute)) {
		return NewValidationError("recorded_at", "cannot be more than one minute in the future")
	}
	return ValidateCoordinates(p.Coordinates, "coordinates")
}
