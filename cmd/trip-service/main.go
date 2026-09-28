package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/keyra88/tripgo/api"
	"github.com/keyra88/tripgo/internal/config"
	"github.com/keyra88/tripgo/internal/database"
	"github.com/keyra88/tripgo/internal/database/txmanager"
	"github.com/keyra88/tripgo/internal/handler"
	"github.com/keyra88/tripgo/internal/repository"
	"github.com/keyra88/tripgo/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	log.Printf("starting trip-service on %s...", cfg.HTTPAddr)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.DBConnectTimeout)
	defer cancel()

	pool, err := database.NewPool(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	log.Println("database connection pool established successfully")

	txMgr := txmanager.New(pool)
	repo := repository.New(pool)
	tripService := service.NewTripService(repo, txMgr)
	tripHandler := handler.New(tripService, pool)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	api.HandlerWithOptions(tripHandler, api.ChiServerOptions{
		BaseRouter: r,
		ErrorHandlerFunc: func(w http.ResponseWriter, req *http.Request, err error) {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(http.StatusBadRequest)
			path := req.URL.Path
			detail := err.Error()
			problem := api.Problem{
				Type:     "https://tripgo.example/problems/invalid-request",
				Title:    "Invalid request",
				Status:   http.StatusBadRequest,
				Code:     "invalid_request",
				Detail:   &detail,
				Instance: &path,
			}
			_ = json.NewEncoder(w).Encode(problem)
		},
	})

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           r,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server listen failed: %v", err)
		}
	}()

	log.Printf("trip-service is listening on %s", cfg.HTTPAddr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	sig := <-stop
	log.Printf("received shutdown signal: %v, gracefully stopping server...", sig)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server graceful shutdown failed: %v", err)
	} else {
		log.Println("http server gracefully stopped")
	}

	log.Println("trip-service exited cleanly")
}