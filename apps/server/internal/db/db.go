// Package db opens the Postgres pool and applies the embedded migrations.
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

// Open creates a connection pool and verifies the database is reachable.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		// The parse error can quote the URL, which contains the password.
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

// Migrator applies and inspects the embedded schema migrations.
type Migrator struct {
	provider *goose.Provider
	db       *sql.DB
	latest   int64
}

// NewMigrator prepares migrations against pool. Up runs under a Postgres session lock, so concurrent
// instances booting together apply each migration exactly once.
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
		_ = sqlDB.Close() // Closing a pool-backed *sql.DB does not close the pool.
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	sources := provider.ListSources()
	if len(sources) == 0 {
		_ = sqlDB.Close()
		return nil, errors.New("no migrations embedded")
	}
	return &Migrator{provider: provider, db: sqlDB, latest: sources[len(sources)-1].Version}, nil
}

// Up applies all pending migrations and returns the resulting schema version.
func (m *Migrator) Up(ctx context.Context) (int64, error) {
	if _, err := m.provider.Up(ctx); err != nil {
		return 0, fmt.Errorf("apply migrations: %w", err)
	}
	return m.provider.GetDBVersion(ctx)
}

// DownTo rolls back to version (used by tests and operators, never at boot).
func (m *Migrator) DownTo(ctx context.Context, version int64) error {
	if _, err := m.provider.DownTo(ctx, version); err != nil {
		return fmt.Errorf("roll back migrations: %w", err)
	}
	return nil
}

// Current reports whether the database schema is at the latest embedded version. It takes no lock.
func (m *Migrator) Current(ctx context.Context) (bool, error) {
	v, err := m.provider.GetDBVersion(ctx)
	if err != nil {
		return false, err
	}
	return v == m.latest, nil
}

// Latest returns the newest embedded migration version.
func (m *Migrator) Latest() int64 { return m.latest }

// Close releases the database/sql handle (the pool stays open).
func (m *Migrator) Close() error { return m.db.Close() }
