package dbtx

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
)

type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func Run(ctx context.Context, db Beginner, fn func(q *sqlc.Queries) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(sqlc.New(tx)); err != nil {
		if rbErr := rollback(ctx, tx); rbErr != nil {
			return errors.Join(err, rbErr)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func rollback(ctx context.Context, tx pgx.Tx) error {
	if err := tx.Rollback(context.WithoutCancel(ctx)); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("rollback transaction: %w", err)
	}
	return nil
}
