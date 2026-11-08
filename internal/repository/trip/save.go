package trip

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
)

func (r *Repository) CreateTrip(ctx context.Context, trip *model.Trip) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	query, args, err := r.sq.Insert("trips").
		Columns(
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
			"created_at",
			"updated_at",
		).
		Values(
			trip.ID,
			trip.UserID,
			trip.DriverID,
			trip.StartPoint.Latitude,
			trip.StartPoint.Longitude,
			trip.EndPoint.Latitude,
			trip.EndPoint.Longitude,
			trip.Price,
			string(trip.Status),
			trip.StartedAt,
			trip.FinishedAt,
			now,
			now,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build create trip sql: %w", err)
	}

	exec := r.getExecutor(ctx)
	_, err = exec.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return domain.ErrDriverBusy
		}
		return fmt.Errorf("exec create trip: %w", err)
	}

	return nil
}

func (r *Repository) UpdateTrip(ctx context.Context, trip *model.Trip) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	now := time.Now().UTC()
	query, args, err := r.sq.Update("trips").
		Set("status", string(trip.Status)).
		Set("finished_at", trip.FinishedAt).
		Set("updated_at", now).
		Where("id = ? AND status = ?", trip.ID, string(model.StatusActive)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update trip sql: %w", err)
	}

	exec := r.getExecutor(ctx)
	cmdTag, err := exec.Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("exec update trip: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrTripCompleted
	}

	return nil
}

func (r *Repository) AddStatusHistory(
	ctx context.Context,
	tripID uuid.UUID,
	fromStatus *string,
	toStatus string,
	reason *string,
	changedAt time.Time,
) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := r.sq.Insert("trip_status_history").
		Columns(
			"trip_id",
			"from_status",
			"to_status",
			"reason",
			"changed_at",
		).
		Values(
			tripID,
			fromStatus,
			toStatus,
			reason,
			changedAt,
		).
		ToSql()
	if err != nil {
		return fmt.Errorf("build status history sql: %w", err)
	}

	exec := r.getExecutor(ctx)
	if _, err := exec.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("exec status history: %w", err)
	}

	return nil
}

func (r *Repository) LockIdempotencyKey(ctx context.Context, key uuid.UUID) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	exec := r.getExecutor(ctx)
	_, err := exec.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", key.String())
	return err
}

func (r *Repository) SaveIdempotencyRecord(ctx context.Context, rec *model.IdempotencyRecord) error {
	ctx, cancel := r.withTimeout(ctx)
	defer cancel()

	query, args, err := r.sq.Insert("idempotency_keys").
		Columns("key", "request_hash", "trip_id", "created_at", "expires_at").
		Values(rec.Key, rec.RequestHash, rec.TripID, rec.CreatedAt, rec.ExpiresAt).
		Suffix("ON CONFLICT (key) DO UPDATE SET request_hash = EXCLUDED.request_hash, trip_id = EXCLUDED.trip_id, created_at = EXCLUDED.created_at, expires_at = EXCLUDED.expires_at").
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert idempotency record sql: %w", err)
	}

	exec := r.getExecutor(ctx)
	if _, err := exec.Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("exec insert idempotency record: %w", err)
	}

	return nil
}
