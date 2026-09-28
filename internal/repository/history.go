package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

func (r *Repository) AddStatusHistory(
	ctx context.Context,
	tripID uuid.UUID,
	fromStatus *string,
	toStatus string,
	reason *string,
	changedAt time.Time,
) error {
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
