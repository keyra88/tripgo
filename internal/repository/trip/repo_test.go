//go:build integration

package trip_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/infra/postgres/transaction"
	"github.com/keyra88/tripgo/internal/model"
	"github.com/keyra88/tripgo/internal/repository/trip"
)

const (
	testConnectTimeout = 5 * time.Second
	testQueryTimeout   = 3 * time.Second
)

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	_ = godotenv.Load()
	_ = godotenv.Load("../../../.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Fatal("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testConnectTimeout)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect to test db: %v", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Fatalf("failed to ping test db: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

func TestRepository_CreateAndGet(t *testing.T) {
	pool := getTestPool(t)
	repo := trip.New(pool, testQueryTimeout)
	ctx := context.Background()

	tripID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	})

	tr := &model.Trip{
		ID:         tripID,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Point{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Point{Latitude: 55.76, Longitude: 37.62},
		Price:      1500,
		Status:     model.StatusActive,
		StartedAt:  now,
		FinishedAt: nil,
	}

	if err := repo.CreateTrip(ctx, tr); err != nil {
		t.Fatalf("failed to create trip: %v", err)
	}

	fetched, err := repo.GetTripByID(ctx, tripID)
	if err != nil {
		t.Fatalf("failed to get trip: %v", err)
	}
	if fetched.ID != tripID || fetched.Price != 1500 || fetched.Status != model.StatusActive {
		t.Errorf("fetched trip mismatch: %+v", fetched)
	}

	secondTripID := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", secondTripID)
	})
	secondTrip := &model.Trip{
		ID:         secondTripID,
		UserID:     uuid.New(),
		DriverID:   driverID, 
		StartPoint: model.Point{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Point{Latitude: 55.76, Longitude: 37.62},
		Price:      2000,
		Status:     model.StatusActive,
		StartedAt:  now,
	}

	err = repo.CreateTrip(ctx, secondTrip)
	if !errors.Is(err, domain.ErrDriverBusy) {
		t.Fatalf("expected ErrDriverBusy, got: %v", err)
	}
}

func TestRepository_GetNotFound(t *testing.T) {
	pool := getTestPool(t)
	repo := trip.New(pool, testQueryTimeout)
	ctx := context.Background()

	_, err := repo.GetTripByID(ctx, uuid.New())
	if !errors.Is(err, domain.ErrTripNotFound) {
		t.Fatalf("expected ErrTripNotFound, got: %v", err)
	}
}

func TestRepository_UpdateTrip(t *testing.T) {
	pool := getTestPool(t)
	repo := trip.New(pool, testQueryTimeout)
	ctx := context.Background()

	tripID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	})

	tr := &model.Trip{
		ID:         tripID,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Point{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Point{Latitude: 55.76, Longitude: 37.62},
		Price:      1000,
		Status:     model.StatusActive,
		StartedAt:  now,
	}

	if err := repo.CreateTrip(ctx, tr); err != nil {
		t.Fatalf("failed to create trip: %v", err)
	}

	finishTime := now.Add(20 * time.Minute)
	tr.Status = model.StatusCompleted
	tr.FinishedAt = &finishTime

	if err := repo.UpdateTrip(ctx, tr); err != nil {
		t.Fatalf("failed to update trip: %v", err)
	}

	fetched, err := repo.GetTripByID(ctx, tripID)
	if err != nil {
		t.Fatalf("failed to get updated trip: %v", err)
	}
	if fetched.Status != model.StatusCompleted || fetched.FinishedAt == nil {
		t.Errorf("expected completed status with finished_at, got: %+v", fetched)
	}
}

func TestRepository_Idempotency(t *testing.T) {
	pool := getTestPool(t)
	repo := trip.New(pool, testQueryTimeout)
	ctx := context.Background()

	key := uuid.New()
	tripID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	now := time.Now().UTC().Truncate(time.Microsecond)

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM idempotency_keys WHERE key = $1", key)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	})

	tr := &model.Trip{
		ID:         tripID,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Point{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Point{Latitude: 55.76, Longitude: 37.62},
		Price:      1000,
		Status:     model.StatusActive,
		StartedAt:  now,
	}
	if err := repo.CreateTrip(ctx, tr); err != nil {
		t.Fatalf("failed to create trip for idempotency: %v", err)
	}

	rec := &model.IdempotencyRecord{
		Key:         key,
		RequestHash: "sample-hash-12345",
		TripID:      tripID,
		CreatedAt:   now,
		ExpiresAt:   now.Add(24 * time.Hour),
	}

	if err := repo.SaveIdempotencyRecord(ctx, rec); err != nil {
		t.Fatalf("failed to save idempotency record: %v", err)
	}

	fetched, err := repo.GetIdempotencyRecord(ctx, key)
	if err != nil {
		t.Fatalf("failed to get idempotency record: %v", err)
	}
	if fetched == nil || fetched.RequestHash != "sample-hash-12345" || fetched.TripID != tripID {
		t.Fatalf("fetched record mismatch: %+v", fetched)
	}

	finishTime := now.Add(10 * time.Minute)
	tr.Status = model.StatusCompleted
	tr.FinishedAt = &finishTime
	if err := repo.UpdateTrip(ctx, tr); err != nil {
		t.Fatalf("failed to finish trip 1: %v", err)
	}

	tripID2 := uuid.New()
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID2)
	})
	tr2 := &model.Trip{
		ID:         tripID2,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Point{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Point{Latitude: 55.76, Longitude: 37.62},
		Price:      1500,
		Status:     model.StatusActive,
		StartedAt:  now,
	}
	if err := repo.CreateTrip(ctx, tr2); err != nil {
		t.Fatalf("failed to create trip2 for idempotency overwrite: %v", err)
	}

	rec2 := &model.IdempotencyRecord{
		Key:         key,
		RequestHash: "sample-hash-67890",
		TripID:      tripID2,
		CreatedAt:   now.Add(25 * time.Hour),
		ExpiresAt:   now.Add(49 * time.Hour),
	}
	if err := repo.SaveIdempotencyRecord(ctx, rec2); err != nil {
		t.Fatalf("failed to overwrite expired idempotency record: %v", err)
	}

	fetched2, err := repo.GetIdempotencyRecord(ctx, key)
	if err != nil {
		t.Fatalf("failed to get updated idempotency record: %v", err)
	}
	if fetched2 == nil || fetched2.RequestHash != "sample-hash-67890" || fetched2.TripID != tripID2 {
		t.Fatalf("updated record mismatch: %+v", fetched2)
	}
}

func TestRepository_TransactionRollback(t *testing.T) {
	pool := getTestPool(t)
	repo := trip.New(pool, testQueryTimeout)
	txMgr := transaction.New(pool, testQueryTimeout)
	ctx := context.Background()

	tripID := uuid.New()
	userID := uuid.New()
	driverID := uuid.New()
	now := time.Now().UTC()

	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, "DELETE FROM trip_status_history WHERE trip_id = $1", tripID)
		_, _ = pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", tripID)
	})

	tr := &model.Trip{
		ID:         tripID,
		UserID:     userID,
		DriverID:   driverID,
		StartPoint: model.Point{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   model.Point{Latitude: 55.76, Longitude: 37.62},
		Price:      1200,
		Status:     model.StatusActive,
		StartedAt:  now,
	}

	expectedErr := errors.New("simulated error to trigger rollback")

	err := txMgr.Do(ctx, func(txCtx context.Context) error {
		if err := repo.CreateTrip(txCtx, tr); err != nil {
			return err
		}
		if err := repo.AddStatusHistory(txCtx, tripID, nil, string(model.StatusActive), nil, now); err != nil {
			return err
		}
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	var countTrips int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM trips WHERE id = $1", tripID).Scan(&countTrips)
	if err != nil {
		t.Fatalf("query trips count failed: %v", err)
	}
	if countTrips != 0 {
		t.Errorf("expected 0 trips (rollback), got %d", countTrips)
	}

	var countHistory int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM trip_status_history WHERE trip_id = $1", tripID).Scan(&countHistory)
	if err != nil {
		t.Fatalf("query history count failed: %v", err)
	}
	if countHistory != 0 {
		t.Errorf("expected 0 history records (rollback), got %d", countHistory)
	}
}

func TestRepository_CountActiveTrips(t *testing.T) {
	pool := getTestPool(t)
	repo := trip.New(pool, testQueryTimeout)
	ctx := context.Background()

	initialCount, err := repo.CountActiveTrips(ctx)
	if err != nil {
		t.Fatalf("failed to count active trips: %v", err)
	}

	tripID := uuid.New()
	driverID := uuid.New()
	tr := &model.Trip{
		ID:         tripID,
		UserID:     uuid.New(),
		DriverID:   driverID,
		StartPoint: model.Point{Latitude: 59.9, Longitude: 30.3},
		EndPoint:   model.Point{Latitude: 59.8, Longitude: 30.2},
		Price:      1000,
		Status:     model.StatusActive,
		StartedAt:  time.Now().UTC(),
	}

	if err := repo.CreateTrip(ctx, tr); err != nil {
		t.Fatalf("failed to create trip: %v", err)
	}

	afterCreate, err := repo.CountActiveTrips(ctx)
	if err != nil {
		t.Fatalf("failed to count active trips after create: %v", err)
	}
	if afterCreate != initialCount+1 {
		t.Errorf("expected count to increase by 1, got initial %d, after %d", initialCount, afterCreate)
	}

	finishedAt := time.Now().UTC()
	tr.Status = model.StatusCompleted
	tr.FinishedAt = &finishedAt
	if err := repo.UpdateTrip(ctx, tr); err != nil {
		t.Fatalf("failed to update trip: %v", err)
	}

	afterFinish, err := repo.CountActiveTrips(ctx)
	if err != nil {
		t.Fatalf("failed to count active trips after finish: %v", err)
	}
	if afterFinish != initialCount {
		t.Errorf("expected count to return to initial %d, got %d", initialCount, afterFinish)
	}
}
