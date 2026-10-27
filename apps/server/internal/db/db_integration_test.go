package db

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/api"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/api/gen"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/config"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/db/sqlc"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/server"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/web"
)

func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DHOW_TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("DHOW_TEST_DATABASE_URL must be set in CI")
		}
		t.Skip("DHOW_TEST_DATABASE_URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newTestMigrator(t *testing.T, pool *pgxpool.Pool) *Migrator {
	t.Helper()
	m, err := NewMigrator(pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Errorf("close migrator: %v", err)
		}
	})
	return m
}

func TestMigrationsAndReadiness(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	m := newTestMigrator(t, pool)

	if err := m.DownTo(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if ok, err := m.Current(ctx); err != nil || ok {
		t.Fatalf("Current on empty schema = %v, %v", ok, err)
	}

	router := server.Router(server.Options{
		Config: config.Config{PublicURL: &url.URL{Scheme: "https", Host: "dhow.example.com"}},
		API: api.New(api.Deps{
			DB: pool, Migrations: m, Queries: sqlc.New(pool),
			Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		}),
		Web: web.Handler(),
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody))
		return rec
	}

	if rec := get("/api/v1/health/ready"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready before migrations = %d", rec.Code)
	}

	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mm, err := NewMigrator(pool)
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
		t.Fatalf("concurrent Up: %v", err)
	}

	if v, err := m.Up(ctx); err != nil || v != m.Latest() {
		t.Fatalf("second Up = %d, %v (latest %d)", v, err, m.Latest())
	}
	if rec := get("/api/v1/health/ready"); rec.Code != http.StatusOK {
		t.Fatalf("ready after migrations = %d: %s", rec.Code, rec.Body.String())
	}

	rec := get("/api/v1/meta")
	var meta gen.MetaResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || !meta.Success || meta.Data == nil || !meta.Data.SetupRequired {
		t.Fatalf("meta = %d %s, want SetupRequired on a fresh instance", rec.Code, rec.Body.String())
	}

	if _, err := pool.Exec(ctx, `INSERT INTO "Instance" ("Id") VALUES (2)`); err == nil {
		t.Fatal("instance table accepted a second row")
	}
}
