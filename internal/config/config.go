package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr              string
	DatabaseURL           string
	LogLevel              string
	ShutdownTimeout       time.Duration
	HTTPReadTimeout       time.Duration
	HTTPReadHeaderTimeout time.Duration
	HTTPWriteTimeout      time.Duration
	HTTPIdleTimeout       time.Duration

	DBMaxConns        int32
	DBMinConns        int32
	DBMaxConnLifetime time.Duration
	DBConnectTimeout  time.Duration
	DBQueryTimeout    time.Duration
}

func Load() (*Config, error) {
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	httpAddr := getEnv("HTTP_ADDR", ":8080")
	if httpAddr == "" {
		return nil, fmt.Errorf("HTTP_ADDR cannot be empty")
	}

	shutdownTimeout, err := parseEnvDuration("SHUTDOWN_TIMEOUT", "10s")
	if err != nil {
		return nil, err
	}

	httpReadTimeout, err := parseEnvDuration("HTTP_READ_TIMEOUT", "5s")
	if err != nil {
		return nil, err
	}

	httpReadHeaderTimeout, err := parseEnvDuration("HTTP_READ_HEADER_TIMEOUT", "2s")
	if err != nil {
		return nil, err
	}

	httpWriteTimeout, err := parseEnvDuration("HTTP_WRITE_TIMEOUT", "10s")
	if err != nil {
		return nil, err
	}

	httpIdleTimeout, err := parseEnvDuration("HTTP_IDLE_TIMEOUT", "120s")
	if err != nil {
		return nil, err
	}

	maxConns, err := parseEnvInt32("DATABASE_MAX_CONNS", 10)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_MAX_CONNS: %w", err)
	}
	if maxConns <= 0 {
		return nil, fmt.Errorf("DATABASE_MAX_CONNS must be positive, got %d", maxConns)
	}

	minConns, err := parseEnvInt32("DATABASE_MIN_CONNS", 2)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_MIN_CONNS: %w", err)
	}
	if minConns < 0 {
		return nil, fmt.Errorf("DATABASE_MIN_CONNS must be non-negative, got %d", minConns)
	}
	if minConns > maxConns {
		return nil, fmt.Errorf("DATABASE_MIN_CONNS (%d) cannot be greater than DATABASE_MAX_CONNS (%d)", minConns, maxConns)
	}

	maxConnLifetime, err := parseEnvDuration("DATABASE_MAX_CONN_LIFETIME", "30m")
	if err != nil {
		return nil, err
	}

	connTimeout, err := parseEnvDuration("DATABASE_CONNECT_TIMEOUT", "5s")
	if err != nil {
		return nil, err
	}

	queryTimeout, err := parseEnvDuration("DATABASE_QUERY_TIMEOUT", "3s")
	if err != nil {
		return nil, err
	}

	return &Config{
		HTTPAddr:              httpAddr,
		DatabaseURL:           dbURL,
		LogLevel:              getEnv("LOG_LEVEL", "info"),
		ShutdownTimeout:       shutdownTimeout,
		HTTPReadTimeout:       httpReadTimeout,
		HTTPReadHeaderTimeout: httpReadHeaderTimeout,
		HTTPWriteTimeout:      httpWriteTimeout,
		HTTPIdleTimeout:       httpIdleTimeout,

		DBMaxConns:        maxConns,
		DBMinConns:        minConns,
		DBMaxConnLifetime: maxConnLifetime,
		DBConnectTimeout:  connTimeout,
		DBQueryTimeout:    queryTimeout,
	}, nil
}

func getEnv(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func parseEnvDuration(key, def string) (time.Duration, error) {
	val := getEnv(key, def)
	d, err := time.ParseDuration(val)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %v", key, d)
	}
	return d, nil
}

func parseEnvInt32(key string, def int32) (int32, error) {
	val := os.Getenv(key)
	if val == "" {
		return def, nil
	}
	n, err := strconv.ParseInt(val, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(n), nil
}
