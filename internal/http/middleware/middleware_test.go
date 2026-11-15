package middleware_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/http/middleware"
)

func TestRecoverer(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test disaster")
	})

	handler := middleware.Recoverer(logger)(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	var prob api.Problem
	if err := json.Unmarshal(rr.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if prob.Code != "internal_error" {
		t.Errorf("expected code internal_error, got %s", prob.Code)
	}

	logOutput := buf.String()
	if len(logOutput) == 0 {
		t.Errorf("expected log output on panic, got empty")
	}
}

func TestTracingAndLoggingAndMetrics(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	otel.SetTracerProvider(tp)

	metricReader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))
	otel.SetMeterProvider(mp)

	meter := mp.Meter("test")
	duration, _ := meter.Float64Histogram("http.server.request.duration")
	active, _ := meter.Int64UpDownCounter("http.server.active_requests")

	httpMetrics := &middleware.HTTPMetrics{
		Duration:       duration,
		ActiveRequests: active,
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	h := middleware.Recoverer(logger)(
		middleware.Tracing("trip-service")(
			middleware.Logging(logger)(
				middleware.Metrics(httpMetrics)(testHandler),
			),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	spans := spanRecorder.Ended()
	if len(spans) == 0 {
		t.Errorf("expected ended spans, got 0")
	}

	var rm metricdata.ResourceMetrics
	if err := metricReader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect metrics error: %v", err)
	}
	if len(rm.ScopeMetrics) == 0 {
		t.Errorf("expected recorded metrics, got 0")
	}

	logOut := buf.String()
	if len(logOut) == 0 {
		t.Errorf("expected log output, got empty")
	}
}

func TestGetRoutePattern_PreRoutingMatching(t *testing.T) {
	r := chi.NewRouter()

	var recordedActiveRoute string
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			recordedActiveRoute = middleware.GetRoutePattern(req)
			next.ServeHTTP(w, req)
		})
	})

	r.Get("/api/v1/trips/{tripId}", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/123e4567-e89b-12d3-a456-426614174000", nil)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if recordedActiveRoute != "/api/v1/trips/{tripId}" {
		t.Errorf("expected pre-routing pattern '/api/v1/trips/{tripId}', got '%s'", recordedActiveRoute)
	}
}

func TestPanicMiddlewareChain(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	otel.SetTracerProvider(tp)

	metricReader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(metricReader))
	otel.SetMeterProvider(mp)

	meter := mp.Meter("test_panic")
	duration, _ := meter.Float64Histogram("http.server.request.duration")
	active, _ := meter.Int64UpDownCounter("http.server.active_requests")

	httpMetrics := &middleware.HTTPMetrics{
		Duration:       duration,
		ActiveRequests: active,
	}

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom in handler")
	})

	h := middleware.Recoverer(logger)(
		middleware.Tracing("trip-service")(
			middleware.Logging(logger)(
				middleware.Metrics(httpMetrics)(panicHandler),
			),
		),
	)

	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", rr.Code)
	}

	spans := spanRecorder.Ended()
	if len(spans) == 0 {
		t.Fatalf("expected ended span for panic, got 0")
	}
	span := spans[0]
	if span.Status().Code != codes.Error {
		t.Errorf("expected span status Error, got %v", span.Status().Code)
	}

	logOut := buf.String()
	if !strings.Contains(logOut, "panic recovered") {
		t.Errorf("expected 'panic recovered' in logs, got %s", logOut)
	}
	if !strings.Contains(logOut, "request handled") {
		t.Errorf("expected 'request handled' in logs, got %s", logOut)
	}
}
