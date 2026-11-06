package middleware

import (
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type HTTPMetrics struct {
	Duration       metric.Float64Histogram
	ActiveRequests metric.Int64UpDownCounter
}

func Metrics(m *HTTPMetrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m == nil {
				next.ServeHTTP(w, r)
				return
			}

			route := GetRoutePattern(r)
			activeAttrs := metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
			)

			m.ActiveRequests.Add(r.Context(), 1, activeAttrs)
			defer m.ActiveRequests.Add(r.Context(), -1, activeAttrs)

			ww := wrapResponseWriter(w)
			start := time.Now()

			next.ServeHTTP(ww, r)

			durationSec := time.Since(start).Seconds()
			resolvedRoute := GetRoutePattern(r)

			m.Duration.Record(r.Context(), durationSec, metric.WithAttributes(
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", resolvedRoute),
				attribute.Int("http.response.status_code", ww.Status()),
			))
		})
	}
}
