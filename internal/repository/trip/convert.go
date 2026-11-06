package trip

import (
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/keyra88/tripgo/internal/model"
)

func scanTrip(row pgx.Row) (*model.Trip, error) {
	var (
		t          model.Trip
		statusStr  string
		finishedAt *time.Time
	)

	err := row.Scan(
		&t.ID,
		&t.UserID,
		&t.DriverID,
		&t.StartPoint.Latitude,
		&t.StartPoint.Longitude,
		&t.EndPoint.Latitude,
		&t.EndPoint.Longitude,
		&t.Price,
		&statusStr,
		&t.StartedAt,
		&finishedAt,
	)
	if err != nil {
		return nil, err
	}

	t.Status = model.Status(statusStr)
	t.FinishedAt = finishedAt
	return &t, nil
}
