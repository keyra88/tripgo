package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type IdempotencyRecord struct {
	Key         uuid.UUID
	RequestHash string
	TripID      uuid.UUID
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

func (r *Repository) LockIdempotencyKey(ctx context.Context, key uuid.UUID) error {
	exec := r.getExecutor(ctx)
	_, err := exec.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtext($1))", key.String())
	return err
}

func (r *Repository) GetIdempotencyRecord(ctx context.Context, key uuid.UUID) (*IdempotencyRecord, error) {
	exec := r.getExecutor(ctx)

	query, args, err := r.sq.
		Select("key", "request_hash", "trip_id", "created_at", "expires_at").
		From("idempotency_keys").
		Where("key = ?", key).
		ToSql()
	if err != nil {
		return nil, err
	}

	var rec IdempotencyRecord
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
		return nil, err
	}

	return &rec, nil
}

func (r *Repository) SaveIdempotencyRecord(ctx context.Context, rec *IdempotencyRecord) error {
	exec := r.getExecutor(ctx)

	query, args, err := r.sq.
		Insert("idempotency_keys").
		Columns("key", "request_hash", "trip_id", "created_at", "expires_at").
		Values(rec.Key, rec.RequestHash, rec.TripID, rec.CreatedAt, rec.ExpiresAt).
		ToSql()
	if err != nil {
		return err
	}

	_, err = exec.Exec(ctx, query, args...)
	return err
}
