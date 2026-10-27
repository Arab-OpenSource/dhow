package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/Arab-OpenSource/dhow/apps/server/migrations"
)

func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, errors.New("parse database URL: invalid format")
	}
	cfg.MaxConns = 20
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.ConnConfig.RuntimeParams["application_name"] = "dhow-server"

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	return pool, nil
}

type Migrator struct {
	provider *goose.Provider
	db       *sql.DB
	latest   int64
}

func NewMigrator(pool *pgxpool.Pool) (*Migrator, error) {
	return newMigrator(pool, migrations.FS)
}

func newMigrator(pool *pgxpool.Pool, fsys fs.FS) (*Migrator, error) {
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return nil, fmt.Errorf("create migration lock: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(pool)
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("load migrations: %w", err), sqlDB.Close())
	}
	sources := provider.ListSources()
	if len(sources) == 0 {
		return nil, errors.Join(errors.New("no migrations embedded"), sqlDB.Close())
	}
	return &Migrator{provider: provider, db: sqlDB, latest: sources[len(sources)-1].Version}, nil
}

func (m *Migrator) Up(ctx context.Context) (int64, error) {
	if _, err := m.provider.Up(ctx); err != nil {
		return 0, fmt.Errorf("apply migrations: %w", err)
	}
	return m.provider.GetDBVersion(ctx)
}

func (m *Migrator) DownTo(ctx context.Context, version int64) error {
	if _, err := m.provider.DownTo(ctx, version); err != nil {
		return fmt.Errorf("roll back migrations: %w", err)
	}
	return nil
}

func (m *Migrator) Current(ctx context.Context) (bool, error) {
	v, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return false, err
	}
	return v == m.latest, nil
}

func (m *Migrator) Latest() int64 { return m.latest }

func (m *Migrator) Close() error { return m.db.Close() }
