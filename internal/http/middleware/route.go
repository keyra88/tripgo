package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

func GetRoutePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx != nil {
		pattern := rctx.RoutePattern()
		if pattern != "" {
			return pattern
		}
	}
	return r.URL.Path
}

type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func wrapResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, status: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func (rw *responseWriter) Status() int {
	return rw.status
}
