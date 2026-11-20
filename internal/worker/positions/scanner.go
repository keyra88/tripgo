package positions

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	resultKey = "result"
	dropped   = "dropped"
)

func (p *Pool) scan(ctx context.Context) {
	ticks := p.newTicker(p.cfg.CheckInterval)
	defer ticks.Stop()
	tickC := ticks.Ticks()
	p.logger.InfoContext(ctx, "stale trip scanner started", slog.Duration("interval", p.cfg.CheckInterval))
	defer p.logger.InfoContext(ctx, "stale trip scanner stopped")

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return
		case <-tickC:
			p.check(ctx)
		}
	}
}

func (p *Pool) check(ctx context.Context) {
	ctx, span := p.tracer.Start(ctx, "scan stale trips", trace.WithNewRoot())
	defer span.End()

	trips, err := p.repo.ListStaleTrips(ctx, p.cfg.StaleAfter)
	p.expireCooldowns()
	if err != nil {
		if ctx.Err() == nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "stale trip query failed")
			p.logger.ErrorContext(ctx, "list stale trips failed", slog.Any("error", err))
		}
		return
	}

	for _, trip := range trips {
		if ctx.Err() != nil {
			return
		}
		result := p.enqueue(ctx, &job{trip: trip, check: span.SpanContext()})
		p.metrics.checks.Add(ctx, 1, metric.WithAttributes(attribute.String(resultKey, result)))
		if result == dropped {
			p.logger.WarnContext(ctx, "position request queue full", slog.String("trip_id", trip.TripID.String()))
		}
	}
}

func (p *Pool) enqueue(ctx context.Context, task *job) string {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := task.trip.TripID
	if _, exists := p.pending[id]; exists || p.now().Before(p.retryAfter[id]) {
		return "duplicate"
	}
	select {
	case p.jobs <- task:
		p.pending[id] = task
		p.metrics.queued.Record(ctx, int64(len(p.jobs)))
		return "queued"
	default:
		return dropped
	}
}

func (p *Pool) expireCooldowns() {
	now := p.now()
	p.mu.Lock()
	defer p.mu.Unlock()

	for id, until := range p.retryAfter {
		if !now.Before(until) {
			delete(p.retryAfter, id)
		}
	}
}
