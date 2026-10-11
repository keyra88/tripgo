package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/keyra88/tripgo/api"
)

func PanicRecoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				log.Printf("PANIC recovered: %v\nstack trace:\n%s", rvr, debug.Stack())

				w.Header().Set("Content-Type", "application/problem+json")
				w.WriteHeader(http.StatusInternalServerError)

				path := r.URL.Path
				detail := "An unexpected error occurred"
				problem := api.Problem{
					Type:     "https://tripgo.example/problems/internal-error",
					Title:    "Internal Server Error",
					Status:   http.StatusInternalServerError,
					Code:     "internal_error",
					Detail:   &detail,
					Instance: &path,
				}
				_ = json.NewEncoder(w).Encode(problem)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
