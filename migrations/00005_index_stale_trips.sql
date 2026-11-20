-- +goose NO TRANSACTION

-- +goose Up
CREATE INDEX CONCURRENTLY trips_active_position_at_idx
    ON trips (COALESCE(last_position_at, started_at), id)
    INCLUDE (driver_id, last_position_at, started_at)
    WHERE status = 'active';

-- +goose Down
DROP INDEX CONCURRENTLY trips_active_position_at_idx;
