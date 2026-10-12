package repository

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/keyra88/tripgo/internal/database/txmanager"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (commandTag pgconn.CommandTag, err error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Repository struct {
	pool *pgxpool.Pool
	sq   squirrel.StatementBuilderType
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool: pool,
		sq:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *Repository) getExecutor(ctx context.Context) DBTX {
	if tx := txmanager.ExtractTx(ctx); tx != nil {
		return tx
	}
	return r.pool
}
