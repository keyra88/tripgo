package config

import (
	"os"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	origDBURL := os.Getenv("DATABASE_URL")
	origReadTimeout := os.Getenv("HTTP_READ_TIMEOUT")
	origMaxConns := os.Getenv("DATABASE_MAX_CONNS")
	origMinConns := os.Getenv("DATABASE_MIN_CONNS")

	defer func() {
		os.Setenv("DATABASE_URL", origDBURL)
		os.Setenv("HTTP_READ_TIMEOUT", origReadTimeout)
		os.Setenv("DATABASE_MAX_CONNS", origMaxConns)
		os.Setenv("DATABASE_MIN_CONNS", origMinConns)
	}()

	t.Run("missing DATABASE_URL", func(t *testing.T) {
		os.Unsetenv("DATABASE_URL")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error when DATABASE_URL is missing, got nil")
		}
	})

	t.Run("zero HTTP_READ_TIMEOUT", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		os.Setenv("HTTP_READ_TIMEOUT", "0s")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error when HTTP_READ_TIMEOUT is 0s, got nil")
		}
	})

	t.Run("negative timeout", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		os.Setenv("HTTP_READ_TIMEOUT", "-5s")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error when HTTP_READ_TIMEOUT is negative, got nil")
		}
	})

	t.Run("zero DATABASE_MAX_CONNS", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		os.Setenv("HTTP_READ_TIMEOUT", "5s")
		os.Setenv("DATABASE_MAX_CONNS", "0")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error when DATABASE_MAX_CONNS is 0, got nil")
		}
	})

	t.Run("minConns greater than maxConns", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		os.Setenv("HTTP_READ_TIMEOUT", "5s")
		os.Setenv("DATABASE_MAX_CONNS", "5")
		os.Setenv("DATABASE_MIN_CONNS", "10")
		_, err := Load()
		if err == nil {
			t.Fatal("expected error when minConns > maxConns, got nil")
		}
	})

	t.Run("valid configuration", func(t *testing.T) {
		os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/db")
		os.Setenv("SHUTDOWN_TIMEOUT", "15s")
		os.Setenv("HTTP_READ_TIMEOUT", "7s")
		os.Setenv("HTTP_READ_HEADER_TIMEOUT", "3s")
		os.Setenv("HTTP_WRITE_TIMEOUT", "12s")
		os.Setenv("HTTP_IDLE_TIMEOUT", "90s")
		os.Setenv("DATABASE_MAX_CONNS", "10")
		os.Setenv("DATABASE_MIN_CONNS", "2")
		os.Setenv("DATABASE_MAX_CONN_LIFETIME", "45m")
		os.Setenv("DATABASE_CONNECT_TIMEOUT", "6s")
		os.Setenv("DATABASE_QUERY_TIMEOUT", "4s")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error for valid config: %v", err)
		}
		if cfg.ShutdownTimeout.Seconds() != 15 {
			t.Errorf("expected 15s ShutdownTimeout, got %v", cfg.ShutdownTimeout)
		}
		if cfg.HTTPReadTimeout.Seconds() != 7 {
			t.Errorf("expected 7s HTTPReadTimeout, got %v", cfg.HTTPReadTimeout)
		}
		if cfg.HTTPReadHeaderTimeout.Seconds() != 3 {
			t.Errorf("expected 3s HTTPReadHeaderTimeout, got %v", cfg.HTTPReadHeaderTimeout)
		}
		if cfg.HTTPWriteTimeout.Seconds() != 12 {
			t.Errorf("expected 12s HTTPWriteTimeout, got %v", cfg.HTTPWriteTimeout)
		}
		if cfg.HTTPIdleTimeout.Seconds() != 90 {
			t.Errorf("expected 90s HTTPIdleTimeout, got %v", cfg.HTTPIdleTimeout)
		}
		if cfg.DBMaxConnLifetime.Minutes() != 45 {
			t.Errorf("expected 45m DBMaxConnLifetime, got %v", cfg.DBMaxConnLifetime)
		}
		if cfg.DBConnectTimeout.Seconds() != 6 {
			t.Errorf("expected 6s DBConnectTimeout, got %v", cfg.DBConnectTimeout)
		}
		if cfg.DBQueryTimeout.Seconds() != 4 {
			t.Errorf("expected 4s DBQueryTimeout, got %v", cfg.DBQueryTimeout)
		}
	})
}
