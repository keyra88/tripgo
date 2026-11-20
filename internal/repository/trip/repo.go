package trip

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/keyra88/tripgo/internal/infra/postgres"
	"github.com/keyra88/tripgo/internal/infra/postgres/transaction"
)

const (
	tripsTable         = "trips"
	positionsTable     = "trip_positions"
	idempotencyTable   = "idempotency_keys"
	statusHistoryTable = "trip_status_history"
)

type Repository struct {
	pool         *pgxpool.Pool
	sq           squirrel.StatementBuilderType
	queryTimeout time.Duration
}

func New(pool *pgxpool.Pool, queryTimeout time.Duration) *Repository {
	return &Repository{
		pool:         pool,
		sq:           squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
		queryTimeout: queryTimeout,
	}
}

func (r *Repository) queryContext(ctx context.Context, collection string) (context.Context, context.CancelFunc) {
	ctx = postgres.WithCollection(ctx, collection)
	if _, ok := ctx.Deadline(); !ok && r.queryTimeout > 0 {
		return context.WithTimeout(ctx, r.queryTimeout)
	}
	return ctx, func() {}
}

func (r *Repository) getExecutor(ctx context.Context) DBTX {
	if tx := transaction.ExtractTx(ctx); tx != nil {
		return tx
	}
	return r.pool
}
