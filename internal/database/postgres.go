package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/keyra88/tripgo/internal/config"
)

func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	poolConfig.MaxConns = cfg.DBMaxConns
	poolConfig.MinConns = cfg.DBMinConns
	poolConfig.MaxConnLifetime = cfg.DBMaxConnLifetime
	poolConfig.ConnConfig.ConnectTimeout = cfg.DBConnectTimeout

	connectCtx, cancel := context.WithTimeout(ctx, cfg.DBConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(connectCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
