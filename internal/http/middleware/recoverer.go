package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/keyra88/tripgo/internal/http/domainerror"
)

type panicCtxKey struct{}

type panicContextHolder struct {
	ctx context.Context
}

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			holder := &panicContextHolder{ctx: r.Context()}
			r = r.WithContext(context.WithValue(r.Context(), panicCtxKey{}, holder))

			defer func() {
				if rvr := recover(); rvr != nil {
					if rvr != http.ErrAbortHandler {
						stack := string(debug.Stack())
						logCtx := r.Context()
						if holder.ctx != nil {
							logCtx = holder.ctx
						}
						if logger != nil {
							logger.ErrorContext(
								logCtx,
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
