package postgres

import (
	"testing"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestQueryDurationCarriesDatabaseLabels(t *testing.T) {
	ctx := t.Context()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	defer func() { require.NoError(t, provider.Shutdown(ctx)) }()

	tracer := newQueryMetricsTracer(provider)
	collection := "trip_positions"
	readCtx := tracer.TraceQueryStart(WithCollection(ctx, collection), nil, pgx.TraceQueryStartData{
		SQL: "SELECT id FROM trip_positions WHERE trip_id = $1",
	})
	tracer.TraceQueryEnd(readCtx, nil, pgx.TraceQueryEndData{})
	writeCtx := tracer.TraceQueryStart(WithCollection(ctx, collection), nil, pgx.TraceQueryStartData{
		SQL: "INSERT INTO trip_positions (trip_id) VALUES ($1)",
	})
	tracer.TraceQueryEnd(writeCtx, nil, pgx.TraceQueryEndData{Err: &pgconn.PgError{Code: pgerrcode.ForeignKeyViolation}})

	var collected metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(ctx, &collected))

	counts := make(map[attribute.Set]uint64)
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name == "db.client.operation.duration" {
				for _, point := range m.Data.(metricdata.Histogram[float64]).DataPoints {
					counts[point.Attributes] += point.Count
				}
			}
		}
	}
	require.Len(t, counts, 2)
	for attrs, count := range counts {
		require.EqualValues(t, 1, count)
		require.True(t, attrs.HasValue("db.collection.name"))
		value, _ := attrs.Value("db.collection.name")
		require.Equal(t, collection, value.AsString())
	}
	for attrs := range counts {
		op, _ := attrs.Value("db.operation.name")
		kind, failed := attrs.Value("error.type")
		if failed {
			require.Equal(t, "INSERT", op.AsString())
			require.Equal(t, pgerrcode.ForeignKeyViolation, kind.AsString())
		} else {
			require.Equal(t, "SELECT", op.AsString())
		}
	}
}
