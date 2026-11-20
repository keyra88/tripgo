package trip

import (
	"context"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/model"
)

const staleTripsSQL = `
SELECT id, driver_id
FROM trips
WHERE status = 'active'
  AND COALESCE(last_position_at, started_at) < now() - $1::interval
ORDER BY COALESCE(last_position_at, started_at), id`

func (r *Repository) UpdateLastPositionAt(ctx context.Context, id uuid.UUID, recordedAt time.Time) error {
	ctx, cancel := r.queryContext(ctx, tripsTable)
	defer cancel()

	query, args, err := r.sq.Update(tripsTable).
		Set("last_position_at", squirrel.Expr("GREATEST(last_position_at, ?)", recordedAt)).
		Set("updated_at", squirrel.Expr("now()")).
		Where("id = ? AND status = ?", id, string(model.StatusActive)).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update last position sql: %w", err)
	}

	tag, err := r.getExecutor(ctx).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update last position: %w", err)
	}
	if tag.RowsAffected() == 0 {
		if _, err := r.GetTripByID(ctx, id); err != nil {
			return err
		}
		return domain.ErrTripCompleted
	}
	return nil
}

func (r *Repository) InsertPosition(ctx context.Context, position model.Position) (*model.Position, error) {
	ctx, cancel := r.queryContext(ctx, positionsTable)
	defer cancel()

	query, args, err := r.sq.Insert(positionsTable).
		Columns("trip_id", "latitude", "longitude", "recorded_at").
		Values(position.TripID, position.Coordinates.Latitude, position.Coordinates.Longitude, position.RecordedAt).
		Suffix("RETURNING id, recorded_at").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build insert position sql: %w", err)
	}
	if err := r.getExecutor(ctx).QueryRow(ctx, query, args...).Scan(&position.ID, &position.RecordedAt); err != nil {
		return nil, fmt.Errorf("insert position: %w", err)
	}
	position.RecordedAt = position.RecordedAt.UTC()
	return &position, nil
}

func (r *Repository) ListPositions(ctx context.Context, id uuid.UUID) ([]model.Position, error) {
	ctx, cancel := r.queryContext(ctx, positionsTable)
	defer cancel()

	query, args, err := r.sq.Select("id", "trip_id", "latitude", "longitude", "recorded_at").
		From(positionsTable).
		Where("trip_id = ?", id).
		OrderBy("recorded_at", "id").
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("build list positions sql: %w", err)
	}
	rows, err := r.getExecutor(ctx).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query positions: %w", err)
	}
	positions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Position, error) {
		var p model.Position
		err := row.Scan(&p.ID, &p.TripID, &p.Coordinates.Latitude, &p.Coordinates.Longitude, &p.RecordedAt)
		p.RecordedAt = p.RecordedAt.UTC()
		return p, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect positions: %w", err)
	}
	return positions, nil
}

func (r *Repository) ListStaleTrips(ctx context.Context, staleAfter time.Duration) ([]model.StaleTrip, error) {
	ctx, cancel := r.queryContext(ctx, tripsTable)
	defer cancel()

	age := pgtype.Interval{Microseconds: staleAfter.Microseconds(), Valid: true}
	rows, err := r.getExecutor(ctx).Query(ctx, staleTripsSQL, age)
	if err != nil {
		return nil, fmt.Errorf("query stale trips: %w", err)
	}
	trips, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.StaleTrip, error) {
		var t model.StaleTrip
		err := row.Scan(&t.TripID, &t.DriverID)
		return t, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect stale trips: %w", err)
	}
	return trips, nil
}
