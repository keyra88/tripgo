package positions

import (
	"fmt"

	"go.opentelemetry.io/otel/metric"
)

type metrics struct {
	checks   metric.Int64Counter
	requests metric.Int64Counter
	duration metric.Float64Histogram
	queued   metric.Int64Gauge
	active   metric.Int64Gauge
}

func newMetrics(meter metric.Meter) (metrics, error) {
	var m metrics
	var err error
	if m.checks, err = meter.Int64Counter("tripgo.stale_checks", metric.WithUnit("{trip}")); err != nil {
		return m, fmt.Errorf("create stale checks counter: %w", err)
	}
	if m.requests, err = meter.Int64Counter("tripgo.position_requests", metric.WithUnit("{request}")); err != nil {
		return m, fmt.Errorf("create position requests counter: %w", err)
	}
	if m.duration, err = meter.Float64Histogram("tripgo.position_requests.duration", metric.WithUnit("s")); err != nil {
		return m, fmt.Errorf("create position requests histogram: %w", err)
	}
	if m.queued, err = meter.Int64Gauge("tripgo.position_requests.queue.size", metric.WithUnit("{request}")); err != nil {
		return m, fmt.Errorf("create position request queue gauge: %w", err)
	}
	if m.active, err = meter.Int64Gauge("tripgo.position_requests.workers.active", metric.WithUnit("{worker}")); err != nil {
		return m, fmt.Errorf("create position workers gauge: %w", err)
	}
	return m, nil
}
