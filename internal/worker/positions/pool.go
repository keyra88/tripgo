package positions

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/keyra88/tripgo/internal/model"
)

type Config struct {
	CheckInterval time.Duration
	StaleAfter    time.Duration
	Workers       int
	QueueSize     int
	PushTimeout   time.Duration
}

type job struct {
	trip    model.StaleTrip
	check   trace.SpanContext
	started bool
}

type Pool struct {
	repo      tripRepository
	push      pushGateway
	cfg       Config
	logger    *slog.Logger
	tracer    trace.Tracer
	metrics   metrics
	newTicker func(time.Duration) ticker
	now       func() time.Time

	mu         sync.Mutex
	jobs       chan *job
	pending    map[uuid.UUID]*job
	retryAfter map[uuid.UUID]time.Time
	active     int64

	stopScanner context.CancelFunc
	stopWorkers context.CancelFunc
	done        chan struct{}
}

func New(repo tripRepository, push pushGateway, cfg Config, logger *slog.Logger, meter metric.Meter) (*Pool, error) {
	m, err := newMetrics(meter)
	if err != nil {
		return nil, err
	}
	return &Pool{
		repo:    repo,
		push:    push,
		cfg:     cfg,
		logger:  logger,
		tracer:  otel.Tracer("tripgo"),
		metrics: m,
		newTicker: func(interval time.Duration) ticker {
			return clockTicker{time.NewTicker(interval)}
		},
		now:        time.Now,
		jobs:       make(chan *job, cfg.QueueSize),
		pending:    make(map[uuid.UUID]*job),
		retryAfter: make(map[uuid.UUID]time.Time),
	}, nil
}

// Start is called once during service startup, before serving HTTP requests.
func (p *Pool) Start(ctx context.Context) {
	scanCtx, stopScanner := context.WithCancel(ctx)
	// A stopped scanner leaves its queued work to the workers.
	workCtx, stopWorkers := context.WithCancel(context.WithoutCancel(ctx))
	p.stopScanner, p.stopWorkers = stopScanner, stopWorkers
	p.done = make(chan struct{})
	go p.run(scanCtx, workCtx)
}

func (p *Pool) run(scanCtx, workCtx context.Context) {
	defer close(p.done)
	defer p.stopScanner()
	defer p.stopWorkers()
	p.metrics.queued.Record(workCtx, 0)
	p.metrics.active.Record(workCtx, 0)
	p.logger.InfoContext(workCtx, "position worker pool started", slog.Int("workers", p.cfg.Workers))

	var workers sync.WaitGroup
	for range p.cfg.Workers {
		workers.Go(func() { p.work(workCtx) })
	}

	p.scan(scanCtx)
	close(p.jobs)
	workers.Wait()
	p.logger.InfoContext(workCtx, "position worker pool stopped")
}

func (p *Pool) Shutdown(ctx context.Context) error {
	if p.done == nil {
		return nil
	}
	p.stopScanner()
	select {
	case <-p.done:
		return ctx.Err()
	case <-ctx.Done():
		p.stopWorkers()
		<-p.done
		return ctx.Err()
	}
}

func (p *Pool) Cancel(id uuid.UUID) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if task := p.pending[id]; task != nil && !task.started {
		delete(p.pending, id)
	}
}
