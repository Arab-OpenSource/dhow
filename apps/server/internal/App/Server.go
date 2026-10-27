package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	migrations "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Migrations"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	system "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

func Run() error {
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("invalid configuration:\n%w", err)
	}
	log := config.NewLogger(cfg)
	build := buildinfo.Get()
	log.Info("starting dhow-server", "version", build.Version, "commit", build.Commit, "go", build.GoVersion)
	if cfg.Dev {
		log.Warn("DHOW_DEV=true: plain HTTP and unencrypted database connections are allowed. Never use this in production.")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := config.OpenDatabase(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	migrator, err := migrations.NewMigrator(pool)
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

	handler := NewRouter(Options{
		Config: cfg,
		Log:    log,
		System: system.Deps{
			DB:         pool,
			Migrations: migrator,
			Queries:    sqlc.New(pool),
			Build:      build,
			Log:        log,
		},
	})
	return serve(ctx, NewHTTPServer(cfg, handler, log), log, "public_url", cfg.PublicURL.String())
}

func NewHTTPServer(cfg config.Config, handler http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: config.ReadHeaderTimeout,
		ReadTimeout:       config.ReadTimeout,
		WriteTimeout:      config.WriteTimeout,
		IdleTimeout:       config.IdleTimeout,
		MaxHeaderBytes:    config.MaxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
}

func serve(ctx context.Context, srv *http.Server, log *slog.Logger, attrs ...any) error {
	serveErr := make(chan error, 1)
	go func() {
		log.Info("listening", append([]any{"addr", srv.Addr}, attrs...)...)
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

	log.Info("shutting down", "timeout", config.ShutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), config.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("stopped")
	return nil
}
