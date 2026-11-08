package model

import (
	"time"

	"github.com/google/uuid"

	"github.com/keyra88/tripgo/internal/domain/trip"
)

type (
	Trip   = trip.Trip
	Point  = trip.Point
	Status = trip.Status
)

const (
	StatusActive    = trip.StatusActive
	StatusCompleted = trip.StatusCompleted
)

type IdempotencyRecord struct {
	Key         uuid.UUID
	RequestHash string
	TripID      uuid.UUID
	CreatedAt   time.Time
	ExpiresAt   time.Time
}
