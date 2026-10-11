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

	shutdownTimeout, err := time.ParseDuration(getEnv("SHUTDOWN_TIMEOUT", "10s"))
	if err != nil {
		return nil, fmt.Errorf("parse SHUTDOWN_TIMEOUT: %w", err)
	}

	httpReadTimeout, err := time.ParseDuration(getEnv("HTTP_READ_TIMEOUT", "5s"))
	if err != nil {
		return nil, fmt.Errorf("parse HTTP_READ_TIMEOUT: %w", err)
	}

	httpReadHeaderTimeout, err := time.ParseDuration(getEnv("HTTP_READ_HEADER_TIMEOUT", "2s"))
	if err != nil {
		return nil, fmt.Errorf("parse HTTP_READ_HEADER_TIMEOUT: %w", err)
	}

	httpWriteTimeout, err := time.ParseDuration(getEnv("HTTP_WRITE_TIMEOUT", "10s"))
	if err != nil {
		return nil, fmt.Errorf("parse HTTP_WRITE_TIMEOUT: %w", err)
	}

	httpIdleTimeout, err := time.ParseDuration(getEnv("HTTP_IDLE_TIMEOUT", "120s"))
	if err != nil {
		return nil, fmt.Errorf("parse HTTP_IDLE_TIMEOUT: %w", err)
	}

	maxConns, err := parseEnvInt32("DATABASE_MAX_CONNS", 10)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_MAX_CONNS: %w", err)
	}

	minConns, err := parseEnvInt32("DATABASE_MIN_CONNS", 2)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_MIN_CONNS: %w", err)
	}

	maxConnLifetime, err := time.ParseDuration(getEnv("DATABASE_MAX_CONN_LIFETIME", "30m"))
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_MAX_CONN_LIFETIME: %w", err)
	}

	connTimeout, err := time.ParseDuration(getEnv("DATABASE_CONNECT_TIMEOUT", "5s"))
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_CONNECT_TIMEOUT: %w", err)
	}

	queryTimeout, err := time.ParseDuration(getEnv("DATABASE_QUERY_TIMEOUT", "3s"))
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_QUERY_TIMEOUT: %w", err)
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
