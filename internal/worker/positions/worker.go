package positions

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

func (p *Pool) work(ctx context.Context) {
	for task := range p.jobs {
		if ctx.Err() != nil {
			return
		}
		p.process(ctx, task)
	}
}

func (p *Pool) process(ctx context.Context, task *job) {
	ctx, span := p.tracer.Start(ctx, "process position request",
		trace.WithNewRoot(), trace.WithLinks(trace.Link{SpanContext: task.check}))
	defer span.End()

	if !p.begin(ctx, task) {
		p.metrics.requests.Add(ctx, 1, metric.WithAttributes(attribute.String(resultKey, "cancelled")))
		return
	}
	defer p.finish(ctx, task)

	requestCtx, cancel := context.WithTimeout(ctx, p.cfg.PushTimeout)
	defer cancel()
	start := time.Now()
	err := p.push.RequestPosition(requestCtx, task.trip.TripID, task.trip.DriverID)
	elapsed := time.Since(start).Seconds()

	result := "success"
	if err != nil {
		result = "error"
		span.RecordError(err)
		span.SetStatus(codes.Error, "position request failed")
		p.logger.WarnContext(ctx, "request trip position failed",
			slog.String("trip_id", task.trip.TripID.String()), slog.Any("error", err))
	}

	attrs := metric.WithAttributes(attribute.String(resultKey, result))
	p.metrics.requests.Add(ctx, 1, attrs)
	p.metrics.duration.Record(ctx, elapsed, attrs)
}

func (p *Pool) begin(ctx context.Context, task *job) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.metrics.queued.Record(ctx, int64(len(p.jobs)))
	// Match the exact queued job so a stale channel entry cannot revive on a later tick.
	if p.pending[task.trip.TripID] != task {
		return false
	}
	task.started = true
	p.active++
	p.metrics.active.Record(ctx, p.active)
	return true
}

func (p *Pool) finish(ctx context.Context, task *job) {
	p.mu.Lock()
	defer p.mu.Unlock()

	id := task.trip.TripID
	delete(p.pending, id)
	p.retryAfter[id] = p.now().Add(p.cfg.CheckInterval)
	p.active--
	p.metrics.active.Record(ctx, p.active)
}
