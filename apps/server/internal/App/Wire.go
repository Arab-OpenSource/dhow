package app

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	audit "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Audit"
	identity "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity"
	session "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session"
	system "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

type Infra struct {
	Config     config.Config
	Log        *slog.Logger
	Pool       *pgxpool.Pool
	Migrations system.MigrationStatus
	Build      buildinfo.Info
	Breach     auth.BreachChecker
	HashParams *auth.Params
	Now        func() time.Time
	Web        http.Handler
}

type Wired struct {
	Options  Options
	Handler  http.Handler
	Identity *identity.Service
}

func Build(ctx context.Context, in Infra) (*Wired, error) {
	params := auth.DefaultParams()
	if in.HashParams != nil {
		params = *in.HashParams
	}
	hasher, err := auth.NewHasher(ctx, params, runtime.NumCPU())
	if err != nil {
		return nil, err
	}
	breach := in.Breach
	if breach == nil {
		breach = auth.NoBreachCheck{}
		if in.Config.BreachedPasswordCheck {
			breach = auth.NewHIBP(config.BreachedRangeURL, config.BreachedCheckTimeout, nil)
		}
	}
	passwords := auth.NewPasswordPolicy(breach, in.Log, config.PasswordMinLength, config.PasswordMaxLength)
	csrf := auth.NewCSRF(in.Config.SecretKey)
	bus := eventbus.New(in.Log)

	auditService := audit.NewService(in.Pool, in.Log, in.Now)
	audit.Subscribe(bus, auditService)
	identityService := identity.NewService(identity.Options{
		DB: in.Pool, Hasher: hasher, Passwords: passwords, Bus: bus, PublicURL: in.Config.PublicURL.String(), Log: in.Log, Now: in.Now,
	})
	sessionService := session.NewService(session.Options{
		DB: in.Pool, Hasher: hasher, Passwords: passwords, CSRF: csrf, Bus: bus, Log: in.Log, Now: in.Now,
	})

	opts := Options{
		Config: in.Config,
		Log:    in.Log,
		System: system.Deps{
			DB: in.Pool, Migrations: in.Migrations, Queries: sqlc.New(in.Pool), Build: in.Build, Log: in.Log,
		},
		Identity: identityService,
		Session:  sessionService,
		Audit:    auditService,
		CSRF:     csrf,
		Now:      in.Now,
		Web:      in.Web,
	}
	return &Wired{Options: opts, Handler: NewRouter(opts), Identity: identityService}, nil
}
