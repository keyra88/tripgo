package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/domain"
)

func (r *Repository) CreateTrip(ctx context.Context, trip *api.Trip) error {
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
			trip.Id,
			trip.UserId,
			trip.DriverId,
			trip.StartPoint.Latitude,
			trip.StartPoint.Longitude,
			trip.EndPoint.Latitude,
			trip.EndPoint.Longitude,
			trip.Price,
			string(trip.Status),
			trip.StartedAt,
			trip.FinishedAt,
			time.Now().UTC(),
			time.Now().UTC(),
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

func (r *Repository) GetTripByID(ctx context.Context, id uuid.UUID) (*api.Trip, error) {
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

	var (
		trip       api.Trip
		statusStr  string
		finishedAt *time.Time
	)

	err = row.Scan(
		&trip.Id,
		&trip.UserId,
		&trip.DriverId,
		&trip.StartPoint.Latitude,
		&trip.StartPoint.Longitude,
		&trip.EndPoint.Latitude,
		&trip.EndPoint.Longitude,
		&trip.Price,
		&statusStr,
		&trip.StartedAt,
		&finishedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrTripNotFound
		}
		return nil, fmt.Errorf("scan trip: %w", err)
	}

	trip.Status = api.TripStatus(statusStr)
	trip.FinishedAt = finishedAt

	return &trip, nil
}

func (r *Repository) FinishTrip(ctx context.Context, id uuid.UUID, finishedAt time.Time) (*api.Trip, error) {
	query, args, err := r.sq.Update("trips").
		Set("status", string(api.Completed)).
		Set("finished_at", finishedAt).
		Set("updated_at", finishedAt).
		Where("id = ? AND status = ?", id, string(api.Active)).
		Suffix("RETURNING id, user_id, driver_id, start_latitude, start_longitude, end_latitude, end_longitude, price, status, started_at, finished_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build finish trip sql: %w", err)
	}

	exec := r.getExecutor(ctx)
	row := exec.QueryRow(ctx, query, args...)

	var (
		trip       api.Trip
		statusStr  string
		dbFinished *time.Time
	)

	err = row.Scan(
		&trip.Id,
		&trip.UserId,
		&trip.DriverId,
		&trip.StartPoint.Latitude,
		&trip.StartPoint.Longitude,
		&trip.EndPoint.Latitude,
		&trip.EndPoint.Longitude,
		&trip.Price,
		&statusStr,
		&trip.StartedAt,
		&dbFinished,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			existing, getErr := r.GetTripByID(ctx, id)
			if getErr != nil {
				return nil, getErr
			}
			if existing.Status == api.Completed {
				return nil, domain.ErrTripCompleted
			}
			return nil, fmt.Errorf("trip cannot be finished from status %s", existing.Status)
		}
		return nil, fmt.Errorf("scan finished trip: %w", err)
	}

	trip.Status = api.TripStatus(statusStr)
	trip.FinishedAt = dbFinished

	return &trip, nil
}
