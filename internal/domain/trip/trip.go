package trip

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Status string

const (
	StatusActive    Status = "active"
	StatusCompleted Status = "completed"
)

type Point struct {
	Latitude  float64
	Longitude float64
}

type Trip struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	DriverID   uuid.UUID
	StartPoint Point
	EndPoint   Point
	Price      int64
	Status     Status
	StartedAt  time.Time
	FinishedAt *time.Time
}

func ValidateCoordinates(point Point, name string) error {
	if point.Latitude < -90 || point.Latitude > 90 {
		return NewValidationError(name+" latitude", "must be between -90 and 90")
	}
	if point.Longitude < -180 || point.Longitude > 180 {
		return NewValidationError(name+" longitude", "must be between -180 and 180")
	}
	return nil
}

func ValidatePrice(price int64) error {
	if price < 0 {
		return NewValidationError("price", "must be non-negative")
	}
	return nil
}

func (t *Trip) Validate() error {
	if t.UserID == uuid.Nil {
		return NewValidationError("user_id", "cannot be nil")
	}
	if t.DriverID == uuid.Nil {
		return NewValidationError("driver_id", "cannot be nil")
	}
	if err := ValidateCoordinates(t.StartPoint, "start_point"); err != nil {
		return err
	}
	if err := ValidateCoordinates(t.EndPoint, "end_point"); err != nil {
		return err
	}
	if err := ValidatePrice(t.Price); err != nil {
		return err
	}
	if t.Status == StatusActive && t.FinishedAt != nil {
		return errors.New("active trip cannot have finished_at")
	}
	if t.Status == StatusCompleted && t.FinishedAt == nil {
		return errors.New("completed trip must have finished_at")
	}
	return nil
}

func (t *Trip) Finish(at time.Time) error {
	if t.Status != StatusActive {
		return ErrTripCompleted
	}
	t.Status = StatusCompleted
	t.FinishedAt = &at
	return nil
}
