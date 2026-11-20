package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/config"
	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/gateway/push"
	"github.com/keyra88/tripgo/internal/http/domainerror"
	"github.com/keyra88/tripgo/internal/http/middleware"
	triphttp "github.com/keyra88/tripgo/internal/http/trip"
	addpositionhttp "github.com/keyra88/tripgo/internal/http/trip/addposition"
	createhttp "github.com/keyra88/tripgo/internal/http/trip/create"
	finishhttp "github.com/keyra88/tripgo/internal/http/trip/finish"
	gethttp "github.com/keyra88/tripgo/internal/http/trip/get"
	listpositionshttp "github.com/keyra88/tripgo/internal/http/trip/listpositions"
	"github.com/keyra88/tripgo/internal/infra/postgres"
	"github.com/keyra88/tripgo/internal/infra/postgres/transaction"
	"github.com/keyra88/tripgo/internal/infra/telemetry"
	repotrip "github.com/keyra88/tripgo/internal/repository/trip"
	addpositionuc "github.com/keyra88/tripgo/internal/usecase/trip/addposition"
	createuc "github.com/keyra88/tripgo/internal/usecase/trip/create"
	finishuc "github.com/keyra88/tripgo/internal/usecase/trip/finish"
	getuc "github.com/keyra88/tripgo/internal/usecase/trip/get"
	listpositionsuc "github.com/keyra88/tripgo/internal/usecase/trip/listpositions"
	"github.com/keyra88/tripgo/internal/worker/positions"
)

type Container struct {
	Server         *http.Server
	Pool           *pgxpool.Pool
	Logger         *slog.Logger
	telemetry      *telemetry.Telemetry
	positionChecks *positions.Pool
	push           *push.Gateway
}

type tripsCreatedRecorder struct {
	counter metric.Int64Counter
}

func (r *tripsCreatedRecorder) IncTripsCreated(ctx context.Context) {
	if r.counter != nil {
		r.counter.Add(ctx, 1)
	}
}

type tripsCompletedRecorder struct {
	counter metric.Int64Counter
}

func (r *tripsCompletedRecorder) IncTripsCompleted(ctx context.Context) {
	if r.counter != nil {
		r.counter.Add(ctx, 1)
	}
}

type positionsSavedRecorder struct {
	counter metric.Int64Counter
}

func (r *positionsSavedRecorder) IncPositionsSaved(ctx context.Context) {
	r.counter.Add(ctx, 1)
}

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	tel, err := telemetry.Setup(ctx, "trip-service", cfg.ServiceVersion, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("setup telemetry: %w", err)
	}

	pool, err := postgres.NewPool(ctx, cfg)
	if err != nil {
		_ = tel.Shutdown(ctx)
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	txMgr := transaction.New(pool, cfg.DBQueryTimeout)
	repo := repotrip.New(pool, cfg.DBQueryTimeout)

	pushGateway := push.New(cfg.PushHTTPURL, cfg.PushRequestTimeout)
	positionChecks, err := positions.New(repo, pushGateway, positions.Config{
		CheckInterval: cfg.PositionCheckInterval,
		StaleAfter:    cfg.PositionStaleAfter,
		Workers:       cfg.PositionWorkers,
		QueueSize:     cfg.PositionQueueSize,
		PushTimeout:   cfg.PushRequestTimeout,
	}, tel.Logger, otel.Meter("tripgo"))
	if err != nil {
		pushGateway.Close()
		pool.Close()
		_ = tel.Shutdown(ctx)
		return nil, fmt.Errorf("create position worker pool: %w", err)
	}

	_, _ = tel.RegisterActiveTripsCallback(repo.CountActiveTrips)

	createdRec := &tripsCreatedRecorder{counter: tel.Metrics.TripsCreated}
	completedRec := &tripsCompletedRecorder{counter: tel.Metrics.TripsCompleted}

	positionsRec := &positionsSavedRecorder{counter: tel.Metrics.PositionsSaved}

	createUC := createuc.New(repo, repo, repo, txMgr, createdRec, tel.Logger)
	getUC := getuc.New(repo, tel.Logger)
	finishUC := finishuc.New(repo, repo, txMgr, completedRec, tel.Logger)
	addPositionUC := addpositionuc.New(repo, txMgr, positionChecks, positionsRec, time.Now)
	listPositionsUC := listpositionsuc.New(repo)

	createH := createhttp.New(createUC, tel.Logger)
	getH := gethttp.New(getUC, cfg.DBQueryTimeout, tel.Logger)
	finishH := finishhttp.New(finishUC, tel.Logger)
	addPositionH := addpositionhttp.New(addPositionUC, tel.Logger)
	listPositionsH := listpositionshttp.New(listPositionsUC, tel.Logger)
	serverH := triphttp.NewServer(createH, getH, finishH, addPositionH, listPositionsH, pool, cfg.DBQueryTimeout)

	r := chi.NewRouter()

	r.Use(middleware.Recoverer(tel.Logger))
	r.Use(middleware.Tracing("trip-service"))
	r.Use(middleware.Logging(tel.Logger))
	r.Use(middleware.Metrics(&middleware.HTTPMetrics{
		Duration:       tel.Metrics.HTTPDuration,
		ActiveRequests: tel.Metrics.HTTPActiveRequests,
	}))

	api.HandlerWithOptions(serverH, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, req *http.Request, err error) {
			domainerror.Write(w, req, fmt.Errorf("%w: %s", domain.ErrInvalidRequest, err.Error()))
		},
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadTimeout:       cfg.HTTPReadTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
	}

	return &Container{
		Server:         srv,
		Pool:           pool,
		Logger:         tel.Logger,
		telemetry:      tel,
		positionChecks: positionChecks,
		push:           pushGateway,
	}, nil
}

func (c *Container) Start(ctx context.Context) <-chan error {
	c.positionChecks.Start(ctx)
	serverErr := make(chan error, 1)
	go func() {
		serverErr <- c.Server.ListenAndServe()
	}()
	return serverErr
}

func (c *Container) Shutdown(ctx context.Context) error {
	var errs error
	if err := c.Server.Shutdown(ctx); err != nil {
		errs = errors.Join(errs, fmt.Errorf("http server shutdown: %w", err))
	}
	if err := c.positionChecks.Shutdown(ctx); err != nil {
		errs = errors.Join(errs, fmt.Errorf("position worker pool shutdown: %w", err))
	}
	c.push.Close()
	if c.telemetry != nil && c.telemetry.Shutdown != nil {
		if err := c.telemetry.Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("telemetry shutdown: %w", err))
		}
	}
	c.Pool.Close()
	return errs
}
