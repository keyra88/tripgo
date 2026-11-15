package trip

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
)

func (r *Repository) GetTripByID(ctx context.Context, id uuid.UUID) (*model.Trip, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := r.sq.Select(
		"id",
		"user_id",
		"driver_id",
		"start_latitude",
		"start_longitude",
		"end_latitude",
		"end_longitude",
		"price",
		"status",
		"started_at",
		"finished_at",
	).
		From("trips").
		Where("id = ?", id).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get trip sql: %w", err)
	}

	exec := r.getExecutor(ctx)
	row := exec.QueryRow(ctx, query, args...)

	trip, err := scanTrip(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTripNotFound
		}
		return nil, fmt.Errorf("scan trip: %w", err)
	}

	return trip, nil
}

func (r *Repository) GetIdempotencyRecord(ctx context.Context, key uuid.UUID) (*model.IdempotencyRecord, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	exec := r.getExecutor(ctx)

	query, args, err := r.sq.
		Select("key", "request_hash", "trip_id", "created_at", "expires_at").
		From("idempotency_keys").
		Where("key = ?", key).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build get idempotency sql: %w", err)
	}

	var rec model.IdempotencyRecord
	err = exec.QueryRow(ctx, query, args...).Scan(
		&rec.Key,
		&rec.RequestHash,
		&rec.TripID,
		&rec.CreatedAt,
		&rec.ExpiresAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("scan idempotency record: %w", err)
	}

	return &rec, nil
}

func (r *Repository) CountActiveTrips(ctx context.Context) (int64, error) {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := r.sq.
		Select("COUNT(*)").
		From("trips").
		Where("status = ?", string(model.StatusActive)).
		ToSql()
	if err != nil {
		return 0, fmt.Errorf("build count active trips sql: %w", err)
	}

	exec := r.getExecutor(ctx)
	var count int64
	if err := exec.QueryRow(ctx, query, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}
