package service

import "github.com/keyra88/tripgo/internal/domain"

var (
	ErrDriverBusy     = domain.ErrDriverBusy
	ErrTripNotFound   = domain.ErrTripNotFound
	ErrTripCompleted       = domain.ErrTripCompleted
	ErrInvalidRequest      = domain.ErrInvalidRequest
	ErrIdempotencyConflict = domain.ErrIdempotencyConflict
)
