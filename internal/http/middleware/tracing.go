package middleware

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

func Tracing(serviceName string) func(http.Handler) http.Handler {
	tracer := otel.GetTracerProvider().Tracer(serviceName)
	propagator := otel.GetTextMapPropagator()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))

			ctx, span := tracer.Start(ctx, r.Method, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			ww := wrapResponseWriter(w)
			r = r.WithContext(ctx)

			next.ServeHTTP(ww, r)

			route := GetRoutePattern(r)
			spanName := fmt.Sprintf("%s %s", r.Method, route)
			span.SetName(spanName)

			span.SetAttributes(
				semconv.HTTPRequestMethodKey.String(r.Method),
				semconv.HTTPRoute(route),
				semconv.HTTPResponseStatusCode(ww.Status()),
			)

			if ww.Status() >= 500 {
				span.SetStatus(codes.Error, http.StatusText(ww.Status()))
				span.SetAttributes(attribute.String("error.type", http.StatusText(ww.Status())))
			}
		})
	}
}
