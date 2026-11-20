package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type (
	collectionKey     struct{}
	queryErrorTypeKey struct{}
)

// WithCollection associates a query with its SQL table name.
func WithCollection(ctx context.Context, collection string) context.Context {
	if collection == "" {
		return ctx
	}
	return context.WithValue(ctx, collectionKey{}, collection)
}

type queryMetricsTracer struct {
	*otelpgx.Tracer
}

func newQueryMetricsTracer(provider metric.MeterProvider) queryMetricsTracer {
	return queryMetricsTracer{otelpgx.NewTracer(
		otelpgx.WithMetricOperationName(otelpgx.SQLOperationName),
		otelpgx.WithMeterProvider(queryMetricsProvider{provider}),
	)}
}

func (t queryMetricsTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if data.Err != nil {
		ctx = context.WithValue(ctx, queryErrorTypeKey{}, errorType(data.Err))
	}
	t.Tracer.TraceQueryEnd(ctx, conn, data)
}

func errorType(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.SQLState()
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "deadline_exceeded"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return fmt.Sprintf("%T", err)
}

type queryMetricsProvider struct {
	metric.MeterProvider
}

func (p queryMetricsProvider) Meter(name string, opts ...metric.MeterOption) metric.Meter {
	return queryMetricsMeter{p.MeterProvider.Meter(name, opts...)}
}

type queryMetricsMeter struct {
	metric.Meter
}

func (m queryMetricsMeter) Float64Histogram(name string, opts ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	histogram, err := m.Meter.Float64Histogram(name, opts...)
	if err != nil || name != "db.client.operation.duration" {
		return histogram, err
	}
	return queryDuration{histogram}, nil
}

type queryDuration struct {
	metric.Float64Histogram
}

func (h queryDuration) Record(ctx context.Context, value float64, opts ...metric.RecordOption) {
	// Extend the otelpgx histogram so each operation is still recorded once.
	attrs := make([]attribute.KeyValue, 0, 2)
	if collection, ok := ctx.Value(collectionKey{}).(string); ok {
		attrs = append(attrs, attribute.String("db.collection.name", collection))
	}
	if kind, ok := ctx.Value(queryErrorTypeKey{}).(string); ok {
		attrs = append(attrs, attribute.String("error.type", kind))
	}
	if len(attrs) > 0 {
		opts = append(opts, metric.WithAttributeSet(attribute.NewSet(attrs...)))
	}
	h.Float64Histogram.Record(ctx, value, opts...)
}
