//go:build integration

package trip_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/keyra88/tripgo/internal/infra/postgres/transaction"
	"github.com/keyra88/tripgo/internal/model"
	"github.com/keyra88/tripgo/internal/repository/trip"
	"github.com/keyra88/tripgo/internal/usecase/trip/addposition"
)

func TestRepositoryListsOnlyStaleActiveTripsOldestFirst(t *testing.T) {
	pool := getTestPool(t)
	repo := trip.New(pool, testQueryTimeout)
	txm := transaction.New(pool, testQueryTimeout)

	err := txm.Do(t.Context(), func(ctx context.Context) error {
		var now time.Time
		require.NoError(t, transaction.ExtractTx(ctx).QueryRow(ctx, "SELECT now()").Scan(&now))
		oldest := seedTrip(t, ctx, repo, now.Add(-10*time.Minute))
		stale := seedTrip(t, ctx, repo, now.Add(-15*time.Minute))
		require.NoError(t, repo.UpdateLastPositionAt(ctx, stale.ID, now.Add(-2*time.Minute)))
		fresh := seedTrip(t, ctx, repo, now.Add(-time.Hour))
		require.NoError(t, repo.UpdateLastPositionAt(ctx, fresh.ID, now.Add(-30*time.Second)))
		completed := seedTrip(t, ctx, repo, now.Add(-time.Hour))
		require.NoError(t, completed.Finish(now))
		require.NoError(t, repo.UpdateTrip(ctx, completed))
		seedTrip(t, ctx, repo, now.Add(-time.Minute))

		trips, err := repo.ListStaleTrips(ctx, time.Minute)
		require.NoError(t, err)
		require.Equal(t, []model.StaleTrip{
			{TripID: oldest.ID, DriverID: oldest.DriverID},
			{TripID: stale.ID, DriverID: stale.DriverID},
		}, trips)
		return nil
	})
	require.NoError(t, err)
}

func seedTrip(t *testing.T, ctx context.Context, repo *trip.Repository, startedAt time.Time) *model.Trip {
	t.Helper()
	tr := &model.Trip{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		DriverID:  uuid.New(),
		Status:    model.StatusActive,
		StartedAt: startedAt,
	}
	require.NoError(t, repo.CreateTrip(ctx, tr))
	return tr
}

type failingPositionRepository struct {
	*trip.Repository
	err error
}

func (r failingPositionRepository) InsertPosition(ctx context.Context, p model.Position) (*model.Position, error) {
	if _, err := r.Repository.InsertPosition(ctx, p); err != nil {
		return nil, err
	}
	return nil, r.err
}

func TestSavePositionRollsBackTripAndPositionOnError(t *testing.T) {
	pool := getTestPool(t)
	repo := trip.New(pool, testQueryTimeout)
	txm := transaction.New(pool, testQueryTimeout)
	ctx := t.Context()
	now := time.Now().UTC().Truncate(time.Microsecond)
	tr := seedTrip(t, ctx, repo, now.Add(-time.Minute))
	failure := errors.New("simulated failure after inserting position")
	uc := addposition.New(failingPositionRepository{Repository: repo, err: failure}, txm, nil, nil, func() time.Time { return now })
	_, err := uc.Execute(ctx, model.Position{TripID: tr.ID, RecordedAt: now})
	require.ErrorIs(t, err, failure)

	stored, err := repo.GetTripByID(ctx, tr.ID)
	require.NoError(t, err)
	require.Nil(t, stored.LastPositionAt)
	points, err := repo.ListPositions(ctx, tr.ID)
	require.NoError(t, err)
	require.Empty(t, points)
}
