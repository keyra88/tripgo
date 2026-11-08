package middleware

import (
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/keyra88/tripgo/internal/http/domainerror"
)

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rvr := recover(); rvr != nil {
					if rvr != http.ErrAbortHandler {
						stack := string(debug.Stack())
						if logger != nil {
							logger.ErrorContext(
								r.Context(),
								"panic recovered",
								slog.Any("error", rvr),
								slog.String("stack", stack),
							)
						}
						domainerror.Write(w, r, errors.New("internal server error"))
					}
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
