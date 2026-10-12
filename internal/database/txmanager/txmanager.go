package txmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type txManager struct {
	pool *pgxpool.Pool
	queryTimeout time.Duration
}

func New(pool *pgxpool.Pool, queryTimeout time.Duration) TxManager {
	return &txManager{pool: pool, queryTimeout: queryTimeout}
}

func (tm *txManager) Do(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	if tx := ExtractTx(ctx); tx != nil {
		return fn(ctx)
	}

	opts := pgx.TxOptions{
		IsoLevel: pgx.ReadCommitted,
	}

	beginCtx, beginCancel := context.WithTimeout(ctx, tm.queryTimeout)
	defer beginCancel()

	tx, err := tm.pool.BeginTx(beginCtx, opts)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), tm.queryTimeout)
			_ = tx.Rollback(rollbackCtx)
			rollbackCancel()
			panic(p)
		} else if err != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), tm.queryTimeout)
			_ = tx.Rollback(rollbackCtx)
			rollbackCancel()
		} else if ctxErr := ctx.Err(); ctxErr != nil {
			err = fmt.Errorf("transaction aborted, context canceled: %w", ctxErr)
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), tm.queryTimeout)
			_ = tx.Rollback(rollbackCtx)
			rollbackCancel()
		} else {
			commitCtx, commitCancel := context.WithTimeout(ctx, tm.queryTimeout)
			if commitErr := tx.Commit(commitCtx); commitErr != nil {
				err = fmt.Errorf("commit transaction: %w", commitErr)
				rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), tm.queryTimeout)
				_ = tx.Rollback(rollbackCtx)
				rollbackCancel()
			}
			commitCancel()
		}
	}()

	txCtx := ctx
	var txCancel context.CancelFunc
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		txCtx, txCancel = context.WithTimeout(ctx, tm.queryTimeout)
		defer txCancel()
	}

	ctxWithTx := context.WithValue(txCtx, txKey{}, tx)
	return fn(ctxWithTx)
}

func ExtractTx(ctx context.Context) pgx.Tx {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return nil
}
