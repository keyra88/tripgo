package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

type Metrics struct {
	HTTPDuration       metric.Float64Histogram
	HTTPActiveRequests metric.Int64UpDownCounter
	TripsCreated       metric.Int64Counter
	TripsCompleted     metric.Int64Counter
	TripsActive        metric.Int64ObservableGauge
}

type Telemetry struct {
	Logger   *slog.Logger
	Metrics  *Metrics
	Shutdown func(context.Context) error
	meter    metric.Meter
}

type traceContextHandler struct {
	handler slog.Handler
}

func (h *traceContextHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *traceContextHandler) Handle(ctx context.Context, record slog.Record) error {
	span := trace.SpanFromContext(ctx)
	if span.SpanContext().IsValid() {
		record.AddAttrs(
			slog.String("trace_id", span.SpanContext().TraceID().String()),
			slog.String("span_id", span.SpanContext().SpanID().String()),
		)
	}
	return h.handler.Handle(ctx, record)
}

func (h *traceContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &traceContextHandler{handler: h.handler.WithAttrs(attrs)}
}

func (h *traceContextHandler) WithGroup(name string) slog.Handler {
	return &traceContextHandler{handler: h.handler.WithGroup(name)}
}

func Setup(ctx context.Context, serviceName, logLevel string) (*Telemetry, error) {
	var closers []func(context.Context) error
	shutdown := func(shutdownCtx context.Context) error {
		var errs error
		for i := len(closers) - 1; i >= 0; i-- {
			errs = errors.Join(errs, closers[i](shutdownCtx))
		}
		return errs
	}

	var level slog.Level
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	baseJSONHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	})
	wrappedHandler := &traceContextHandler{
		handler: baseJSONHandler.WithAttrs([]slog.Attr{
			slog.String("service", serviceName),
		}),
	}
	logger := slog.New(wrappedHandler)

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			attribute.String("service.namespace", "tripgo"),
			attribute.String("deployment.environment", "local"),
		),
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
	)
	if err != nil {
		return nil, err
	}

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	var tp *sdktrace.TracerProvider
	if endpoint != "" {
		traceOpts := []otlptracehttp.Option{
			otlptracehttp.WithTimeout(3 * time.Second),
		}
		if !strings.HasPrefix(endpoint, "https://") {
			traceOpts = append(traceOpts, otlptracehttp.WithInsecure())
		}
		if strings.HasPrefix(endpoint, "http://") {
			trimmed := strings.TrimPrefix(endpoint, "http://")
			traceOpts = append(traceOpts, otlptracehttp.WithEndpoint(trimmed))
		} else if !strings.HasPrefix(endpoint, "https://") {
			traceOpts = append(traceOpts, otlptracehttp.WithEndpoint(endpoint))
		}

		traceExporter, err := otlptracehttp.New(ctx, traceOpts...)
		if err == nil {
			tp = sdktrace.NewTracerProvider(
				sdktrace.WithResource(res),
				sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(time.Second)),
			)
			closers = append(closers, tp.Shutdown)
		}
	}

	if tp == nil {
		tp = sdktrace.NewTracerProvider(sdktrace.WithResource(res))
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	var mp *sdkmetric.MeterProvider
	if endpoint != "" {
		metricOpts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithTimeout(3 * time.Second),
		}
		if !strings.HasPrefix(endpoint, "https://") {
			metricOpts = append(metricOpts, otlpmetrichttp.WithInsecure())
		}
		if strings.HasPrefix(endpoint, "http://") {
			trimmed := strings.TrimPrefix(endpoint, "http://")
			metricOpts = append(metricOpts, otlpmetrichttp.WithEndpoint(trimmed))
		} else if !strings.HasPrefix(endpoint, "https://") {
			metricOpts = append(metricOpts, otlpmetrichttp.WithEndpoint(endpoint))
		}

		metricExporter, err := otlpmetrichttp.New(ctx, metricOpts...)
		if err == nil {
			mp = sdkmetric.NewMeterProvider(
				sdkmetric.WithResource(res),
				sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(time.Second))),
			)
			closers = append(closers, mp.Shutdown)
		}
	}

	if mp == nil {
		mp = sdkmetric.NewMeterProvider(sdkmetric.WithResource(res))
	}
	otel.SetMeterProvider(mp)

	meter := mp.Meter("tripgo")

	httpDuration, err := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP requests in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	httpActiveRequests, err := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}

	tripsCreated, err := meter.Int64Counter(
		"tripgo.trips.created",
		metric.WithDescription("Number of trips created"),
		metric.WithUnit("{trip}"),
	)
	if err != nil {
		return nil, err
	}

	tripsCompleted, err := meter.Int64Counter(
		"tripgo.trips.completed",
		metric.WithDescription("Number of trips completed"),
		metric.WithUnit("{trip}"),
	)
	if err != nil {
		return nil, err
	}

	tripsActive, err := meter.Int64ObservableGauge(
		"tripgo.trips.active",
		metric.WithDescription("Number of trips currently in active status"),
		metric.WithUnit("{trip}"),
	)
	if err != nil {
		return nil, err
	}

	return &Telemetry{
		Logger: logger,
		Metrics: &Metrics{
			HTTPDuration:       httpDuration,
			HTTPActiveRequests: httpActiveRequests,
			TripsCreated:       tripsCreated,
			TripsCompleted:     tripsCompleted,
			TripsActive:        tripsActive,
		},
		Shutdown: shutdown,
		meter:    meter,
	}, nil
}

func (t *Telemetry) RegisterActiveTripsCallback(fn func(context.Context) (int64, error)) (metric.Registration, error) {
	if t == nil || t.meter == nil || t.Metrics == nil || t.Metrics.TripsActive == nil || fn == nil {
		return nil, nil
	}
	return t.meter.RegisterCallback(func(ctx context.Context, obs metric.Observer) error {
		count, err := fn(ctx)
		if err == nil {
			obs.ObserveInt64(t.Metrics.TripsActive, count)
		}
		return nil
	}, t.Metrics.TripsActive)
}
