package integration_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	migrations "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Migrations"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	dbtest "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Testing"
	system "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return dbtest.NewEmptyPool(t)
}

func newMigrator(t *testing.T, pool *pgxpool.Pool) *migrations.Migrator {
	t.Helper()
	m, err := migrations.NewMigrator(pool)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, m.Close()) })
	return m
}

func TestMigrationsLifecycle(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	m := newMigrator(t, pool)

	require.NoError(t, m.DownTo(ctx, 0))
	current, err := m.Current(ctx)
	require.NoError(t, err)
	assert.False(t, current, "an empty schema is not current")

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mm, err := migrations.NewMigrator(pool)
			if err != nil {
				errs <- err
				return
			}
			_, upErr := mm.Up(ctx)
			if err := errors.Join(upErr, mm.Close()); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		assert.NoError(t, err, "concurrent Up must serialize on the advisory lock")
	}

	v, err := m.Up(ctx)
	require.NoError(t, err)
	assert.Equal(t, m.Latest(), v, "a second Up is a no-op")
	current, err = m.Current(ctx)
	require.NoError(t, err)
	assert.True(t, current)
}

func TestInstanceSingleton(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	m := newMigrator(t, pool)
	require.NoError(t, m.DownTo(ctx, 0))
	_, err := m.Up(ctx)
	require.NoError(t, err)

	row, err := sqlc.New(pool).GetInstance(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 1, row.Id)
	assert.False(t, row.CreatedAt.IsZero())
	assert.False(t, row.UpdatedAt.IsZero())
	assert.Nil(t, row.SetupCompletedAt)

	_, err = pool.Exec(ctx, `INSERT INTO "Instance" ("Id") VALUES (2)`)
	assert.Error(t, err, "the instance table must reject a second row")
}

func TestMetaServiceAgainstPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	m := newMigrator(t, pool)
	require.NoError(t, m.DownTo(ctx, 0))
	_, err := m.Up(ctx)
	require.NoError(t, err)

	svc := system.NewMetaService(sqlc.New(pool), buildinfo.Info{Version: "v0", Commit: "c"})
	meta, err := svc.Get(ctx)
	require.NoError(t, err)
	assert.True(t, meta.SetupRequired)

	_, err = pool.Exec(ctx, `UPDATE "Instance" SET "SetupCompletedAt" = now(), "UpdatedAt" = now() WHERE "Id" = 1`)
	require.NoError(t, err)
	meta, err = svc.Get(ctx)
	require.NoError(t, err)
	assert.False(t, meta.SetupRequired)
}
