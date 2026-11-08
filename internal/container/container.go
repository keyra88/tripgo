package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/metric"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/config"
	domain "github.com/keyra88/tripgo/internal/domain/trip"
	"github.com/keyra88/tripgo/internal/http/domainerror"
	"github.com/keyra88/tripgo/internal/http/middleware"
	triphttp "github.com/keyra88/tripgo/internal/http/trip"
	createhttp "github.com/keyra88/tripgo/internal/http/trip/create"
	finishhttp "github.com/keyra88/tripgo/internal/http/trip/finish"
	gethttp "github.com/keyra88/tripgo/internal/http/trip/get"
	"github.com/keyra88/tripgo/internal/infra/postgres"
	"github.com/keyra88/tripgo/internal/infra/postgres/transaction"
	"github.com/keyra88/tripgo/internal/infra/telemetry"
	repotrip "github.com/keyra88/tripgo/internal/repository/trip"
	createuc "github.com/keyra88/tripgo/internal/usecase/trip/create"
	finishuc "github.com/keyra88/tripgo/internal/usecase/trip/finish"
	getuc "github.com/keyra88/tripgo/internal/usecase/trip/get"
)

type Container struct {
	Server    *http.Server
	Pool      *pgxpool.Pool
	Logger    *slog.Logger
	telemetry *telemetry.Telemetry
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

func New(ctx context.Context, cfg *config.Config) (*Container, error) {
	tel, err := telemetry.Setup(ctx, "trip-service", cfg.LogLevel)
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

	_, _ = tel.RegisterActiveTripsCallback(repo.CountActiveTrips)

	createdRec := &tripsCreatedRecorder{counter: tel.Metrics.TripsCreated}
	completedRec := &tripsCompletedRecorder{counter: tel.Metrics.TripsCompleted}

	createUC := createuc.New(repo, repo, repo, txMgr, createdRec, tel.Logger)
	getUC := getuc.New(repo, tel.Logger)
	finishUC := finishuc.New(repo, repo, txMgr, completedRec, tel.Logger)

	createH := createhttp.New(createUC)
	getH := gethttp.New(getUC, cfg.DBQueryTimeout)
	finishH := finishhttp.New(finishUC)
	serverH := triphttp.NewServer(createH, getH, finishH, pool, cfg.DBQueryTimeout)

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
		Server:    srv,
		Pool:      pool,
		Logger:    tel.Logger,
		telemetry: tel,
	}, nil
}

func (c *Container) Start() error {
	return c.Server.ListenAndServe()
}

func (c *Container) Shutdown(ctx context.Context) error {
	var errs error
	if err := c.Server.Shutdown(ctx); err != nil {
		errs = errors.Join(errs, fmt.Errorf("http server shutdown: %w", err))
	}
	c.Pool.Close()
	if c.telemetry != nil && c.telemetry.Shutdown != nil {
		if err := c.telemetry.Shutdown(ctx); err != nil {
			errs = errors.Join(errs, fmt.Errorf("telemetry shutdown: %w", err))
		}
	}
	return errs
}
