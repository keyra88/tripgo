package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ww := wrapResponseWriter(w)
			start := time.Now()

			next.ServeHTTP(ww, r)

			duration := time.Since(start)
			route := GetRoutePattern(r)
			status := ww.Status()

			attrs := []slog.Attr{
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.Int("status", status),
				slog.Int64("duration_ms", duration.Milliseconds()),
			}

			if status >= 500 {
				logger.LogAttrs(r.Context(), slog.LevelError, "request handled", attrs...)
			} else {
				logger.LogAttrs(r.Context(), slog.LevelInfo, "request handled", attrs...)
			}
		})
	}
}
