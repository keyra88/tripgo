package positions

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.uber.org/goleak"
	"go.uber.org/mock/gomock"

	"github.com/keyra88/tripgo/internal/model"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type poolFixture struct {
	pool   *Pool
	repo   *MocktripRepository
	push   *MockpushGateway
	ticks  chan time.Time
	reader *sdkmetric.ManualReader
}

func newPoolFixture(t *testing.T, workers, size int) poolFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	repo := NewMocktripRepository(ctrl)
	push := NewMockpushGateway(ctrl)
	tickerMock := NewMockticker(ctrl)
	ticks := make(chan time.Time)

	reader := sdkmetric.NewManualReader()
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { require.NoError(t, meter.Shutdown(context.Background())) })

	pool, err := New(repo, push, Config{
		CheckInterval: 30 * time.Second,
		StaleAfter:    time.Minute,
		Workers:       workers,
		QueueSize:     size,
		PushTimeout:   5 * time.Second,
	}, slog.New(slog.NewJSONHandler(os.Stdout, nil)), meter.Meter("test"))
	require.NoError(t, err)
	pool.newTicker = func(interval time.Duration) ticker {
		if interval != pool.cfg.CheckInterval {
			t.Errorf("ticker interval = %v, want %v", interval, pool.cfg.CheckInterval)
		}
		tickerMock.EXPECT().Ticks().Return(ticks)
		tickerMock.EXPECT().Stop()
		return tickerMock
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, pool.Shutdown(ctx))
	})
	return poolFixture{pool: pool, repo: repo, push: push, ticks: ticks, reader: reader}
}

func (f poolFixture) tick(t *testing.T) {
	t.Helper()
	select {
	case f.ticks <- time.Time{}:
	case <-time.After(5 * time.Second):
		t.Fatal("scanner did not accept the next tick")
	}
}

func (f poolFixture) collect(t *testing.T) map[string]metricdata.Aggregation {
	t.Helper()
	var collected metricdata.ResourceMetrics
	require.NoError(t, f.reader.Collect(t.Context(), &collected))
	metrics := make(map[string]metricdata.Aggregation)
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			metrics[m.Name] = m.Data
		}
	}
	return metrics
}

func resultCounts(data metricdata.Aggregation) map[string]int64 {
	counts := make(map[string]int64)
	for _, point := range data.(metricdata.Sum[int64]).DataPoints {
		result, _ := point.Attributes.Value(resultKey)
		counts[result.AsString()] = point.Value
	}
	return counts
}

func staleTrip() model.StaleTrip {
	return model.StaleTrip{TripID: uuid.New(), DriverID: uuid.New()}
}

func TestScannerDeduplicatesCancelsAndDropsWithoutBlocking(t *testing.T) {
	f := newPoolFixture(t, 1, 2)
	first, cancelled, waiting, overflow := staleTrip(), staleTrip(), staleTrip(), staleTrip()
	gomock.InOrder(
		f.repo.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return([]model.StaleTrip{first}, nil),
		f.repo.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return([]model.StaleTrip{cancelled, waiting, overflow}, nil),
		f.repo.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return([]model.StaleTrip{first, cancelled, waiting}, nil),
		f.repo.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return(nil, nil),
	)

	started := make(chan struct{})
	release := make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), first.TripID, first.DriverID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			close(started)
			select {
			case <-release:
				return errors.New("push unavailable")
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	f.push.EXPECT().RequestPosition(gomock.Any(), waiting.TripID, waiting.DriverID).Return(nil)

	f.pool.Start(t.Context())
	f.tick(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not start")
	}

	f.tick(t)
	f.tick(t)
	f.tick(t) // The scanner accepted another tick with the worker and queue still busy.
	m := f.collect(t)
	require.Equal(t, map[string]int64{"queued": 3, "duplicate": 3, "dropped": 1}, resultCounts(m["tripgo.stale_checks"]))
	require.EqualValues(t, 2, m["tripgo.position_requests.queue.size"].(metricdata.Gauge[int64]).DataPoints[0].Value)

	f.pool.Cancel(cancelled.TripID)
	close(release)
	require.NoError(t, f.pool.Shutdown(t.Context()))
	m = f.collect(t)
	require.Equal(t, map[string]int64{"success": 1, "error": 1, "cancelled": 1}, resultCounts(m["tripgo.position_requests"]))
	require.EqualValues(t, 0, m["tripgo.position_requests.queue.size"].(metricdata.Gauge[int64]).DataPoints[0].Value)
}

func TestPoolDrainsWithFixedWorkersAfterScannerCancellation(t *testing.T) {
	const workers, jobs = 3, 7
	f := newPoolFixture(t, workers, jobs)
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	f.pool.tracer = provider.Tracer("test")
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })

	trips := make([]model.StaleTrip, jobs)
	started := make(chan uuid.UUID, jobs)
	release := make(chan struct{})
	for i := range trips {
		trips[i] = staleTrip()
		f.push.EXPECT().RequestPosition(gomock.Any(), trips[i].TripID, trips[i].DriverID).
			DoAndReturn(func(ctx context.Context, id, _ uuid.UUID) error {
				started <- id
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
	}
	f.repo.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return(trips, nil)
	f.repo.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return(nil, nil)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	f.pool.Start(ctx)
	f.tick(t)
	f.tick(t) // All jobs from the first scan have been offered to the channel.
	for range workers {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("worker did not start")
		}
	}
	m := f.collect(t)
	require.EqualValues(t, workers, m["tripgo.position_requests.workers.active"].(metricdata.Gauge[int64]).DataPoints[0].Value)
	require.EqualValues(t, jobs-workers, m["tripgo.position_requests.queue.size"].(metricdata.Gauge[int64]).DataPoints[0].Value)

	cancel()
	close(release)
	require.NoError(t, f.pool.Shutdown(t.Context()))
	m = f.collect(t)
	require.Equal(t, map[string]int64{"success": jobs}, resultCounts(m["tripgo.position_requests"]))
	require.Len(t, m["tripgo.position_requests.duration"].(metricdata.Histogram[float64]).DataPoints, 1)
	verifyWorkerLinks(t, recorder.Ended(), jobs)
}

func verifyWorkerLinks(t *testing.T, spans []sdktrace.ReadOnlySpan, jobs int) {
	t.Helper()
	var scans []sdktrace.ReadOnlySpan
	var processed []sdktrace.ReadOnlySpan
	for _, span := range spans {
		switch span.Name() {
		case "scan stale trips":
			scans = append(scans, span)
		case "process position request":
			processed = append(processed, span)
		}
	}
	require.Len(t, scans, 2)
	require.Len(t, processed, jobs)
	for _, span := range processed {
		require.False(t, span.Parent().IsValid())
		require.Len(t, span.Links(), 1)
		require.Equal(t, scans[0].SpanContext(), span.Links()[0].SpanContext)
	}
}

func TestShutdownCancellationStopsInFlightRequest(t *testing.T) {
	f := newPoolFixture(t, 1, 1)
	trip := staleTrip()
	f.repo.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return([]model.StaleTrip{trip}, nil)
	started := make(chan struct{})
	f.push.EXPECT().RequestPosition(gomock.Any(), trip.TripID, trip.DriverID).
		DoAndReturn(func(ctx context.Context, _, _ uuid.UUID) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		})

	f.pool.Start(t.Context())
	f.tick(t)
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("worker did not start")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.ErrorIs(t, f.pool.Shutdown(ctx), context.Canceled)
}

func TestScannerWaitsForCooldownAfterDelayedPush(t *testing.T) {
	f := newPoolFixture(t, 1, 1)
	ctx := t.Context()
	now := time.Date(2026, time.November, 20, 12, 0, 0, 0, time.UTC)
	f.pool.now = func() time.Time { return now }
	trip := staleTrip()
	f.repo.EXPECT().ListStaleTrips(gomock.Any(), time.Minute).Return([]model.StaleTrip{trip}, nil).Times(3)
	f.push.EXPECT().RequestPosition(gomock.Any(), trip.TripID, trip.DriverID).Return(nil).Times(2)

	f.pool.check(ctx)
	require.Len(t, f.pool.jobs, 1)
	const queueDelay = 8 * time.Second
	now = now.Add(queueDelay)
	f.pool.process(ctx, <-f.pool.jobs)

	f.pool.Cancel(trip.TripID)
	now = now.Add(f.pool.cfg.CheckInterval - queueDelay)
	f.pool.check(ctx)
	require.Empty(t, f.pool.jobs)
	counts := resultCounts(f.collect(t)["tripgo.stale_checks"])
	require.EqualValues(t, 1, counts["duplicate"])

	now = now.Add(queueDelay)
	f.pool.check(ctx)
	require.Len(t, f.pool.jobs, 1)
	require.Empty(t, f.pool.retryAfter)
	f.pool.process(ctx, <-f.pool.jobs)
}
