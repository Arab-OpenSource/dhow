package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/api"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/config"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/db"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/db/sqlc"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/logging"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/server"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/web"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

const shutdownTimeout = 20 * time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "dhow-server:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}
	log := logging.New(os.Stdout, cfg.LogLevel)
	build := buildinfo.Get()
	log.Info("starting dhow-server", "version", build.Version, "commit", build.Commit, "go", build.GoVersion)
	if cfg.Dev {
		log.Warn("DHOW_DEV=true: plain HTTP and unencrypted database connections are allowed. Never use this in production.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrator, err := db.NewMigrator(pool)
	if err != nil {
		return err
	}
	defer func() {
		if err := migrator.Close(); err != nil {
			log.Warn("close migrator", "error", err)
		}
	}()
	version, err := migrator.Up(ctx)
	if err != nil {
		return err
	}
	log.Info("database schema is current", "version", version)

	srv := server.New(server.Options{
		Config: cfg,
		API: api.New(api.Deps{
			DB:         pool,
			Migrations: migrator,
			Queries:    sqlc.New(pool),
			Build:      build,
			Log:        log,
		}),
		Web: web.Handler(),
		Log: log,
	})

	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", cfg.ListenAddr, "public_url", cfg.PublicURL.String())
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	log.Info("shutting down", "timeout", shutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("stopped")
	return nil
}
