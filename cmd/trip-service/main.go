package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/keyra88/tripgo/internal/config"
	"github.com/keyra88/tripgo/internal/container"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	cnt, err := container.New(ctx, cfg)
	if err != nil {
		log.Fatalf("failed to create container: %v", err)
	}

	cnt.Logger.Info("starting trip-service", slog.String("addr", cfg.HTTPAddr))

	go func() {
		if err := cnt.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			cnt.Logger.Error("server failed to listen", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	sig := <-stop
	cnt.Logger.Info("received shutdown signal, stopping gracefully...", slog.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- cnt.Shutdown(shutdownCtx)
	}()

	select {
	case <-shutdownCtx.Done():
		shutdownCancel()
		cnt.Logger.Error("shutdown timeout exceeded, force exiting", slog.Duration("timeout", cfg.ShutdownTimeout))
		os.Exit(1)
	case err := <-shutdownDone:
		shutdownCancel()
		if err != nil {
			cnt.Logger.Error("graceful shutdown failed", slog.Any("error", err))
			os.Exit(1)
		}
	}

	cnt.Logger.Info("trip-service stopped gracefully")
}
