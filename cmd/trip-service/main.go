package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/keyra88/tripgo/internal/config"
	"github.com/keyra88/tripgo/internal/container"
	"github.com/keyra88/tripgo/internal/infra/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		failStartup("failed to load config", err)
	}

	ctx := context.Background()

	cnt, err := container.New(ctx, cfg)
	if err != nil {
		failStartup("failed to create container", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	cnt.Logger.Info("starting trip-service", slog.String("addr", cfg.HTTPAddr))
	serverErr := cnt.Start(ctx)

	var failed bool
	select {
	case sig := <-stop:
		cnt.Logger.Info("received shutdown signal, stopping gracefully", slog.String("signal", sig.String()))
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			cnt.Logger.Error("server failed to listen", slog.Any("error", err))
			failed = true
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, cfg.ShutdownTimeout)

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

	signal.Stop(stop)
	cnt.Logger.Info("trip-service stopped gracefully")
	if failed {
		os.Exit(1)
	}
}

func failStartup(message string, err error) {
	logger := telemetry.NewLogger("trip-service", os.Getenv("LOG_LEVEL"))
	logger.Error(message, slog.Any("error", err))
	os.Exit(1)
}
