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
	appcrypto "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Crypto"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	mailer "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Mail"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	runnerauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/RunnerAuth"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	audit "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Audit"
	identity "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity"
	runners "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Runners"
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
	Mailer     mailer.Sender
}

type Wired struct {
	Options       Options
	Handler       http.Handler
	RunnerHandler http.Handler
	Identity      *identity.Service
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
	sealer, err := appcrypto.NewSealer(in.Config.SecretKey, config.TotpSealLabel)
	if err != nil {
		return nil, err
	}
	recovery, err := auth.NewRecoveryHasher(in.Config.SecretKey)
	if err != nil {
		return nil, err
	}
	passkeys, err := auth.NewWebAuthn(in.Config.PublicURL, config.WebAuthnDisplayName, config.WebAuthnChallengeTTL)
	if err != nil {
		return nil, err
	}
	runnerKeys, err := runnerauth.DeriveKeys(in.Config.SecretKey)
	if err != nil {
		return nil, err
	}
	bus := eventbus.New(in.Log)
	instance := policy.InstanceSettings{AnyUserCreatesOrganizations: in.Config.AnyUserCreatesOrganizations}

	auditService := audit.NewService(in.Pool, in.Log, in.Now)
	audit.Subscribe(bus, auditService)
	sessionService := session.NewService(session.Options{
		DB: in.Pool, Hasher: hasher, Passwords: passwords, CSRF: csrf, Sealer: sealer, Recovery: recovery, WebAuthn: passkeys,
		Bus: bus, Instance: instance, Log: in.Log, Now: in.Now,
	})
	mail := in.Mailer
	if mail == nil {
		mail = mailerFor(in.Config, in.Now)
	}
	identityService := identity.NewService(identity.Options{
		DB: in.Pool, Hasher: hasher, Passwords: passwords, Bus: bus, Factors: sessionService, Mailer: mail,
		PublicURL: in.Config.PublicURL.String(), Log: in.Log, Now: in.Now,
	})

	runnersService := runners.NewService(runners.Options{
		DB: in.Pool, Keys: runnerKeys, Bus: bus, Origin: auth.OriginOf(in.Config.PublicURL), PublicURL: in.Config.PublicURL.String(),
		Log: in.Log, Now: in.Now,
	})
	gate := runnerauth.NewGate(runnerauth.GateOptions{Verifier: runnersService, Log: in.Log, Now: in.Now})

	opts := Options{
		Config: in.Config,
		Log:    in.Log,
		System: system.Deps{
			DB: in.Pool, Migrations: in.Migrations, Queries: sqlc.New(in.Pool), Build: in.Build, Log: in.Log,
		},
		Identity: identityService,
		Session:  sessionService,
		Audit:    auditService,
		Runners:  runnersService,
		Gate:     gate,
		CSRF:     csrf,
		Now:      in.Now,
		Web:      in.Web,
	}
	wired := &Wired{Options: opts, Handler: NewRouter(opts), Identity: identityService}
	if in.Config.RunnerPlaneSeparate() {
		wired.RunnerHandler = RunnerPlaneServer(opts)
	}
	return wired, nil
}

func mailerFor(cfg config.Config, now func() time.Time) mailer.Sender {
	if !cfg.SMTP.Enabled() {
		return mailer.Disabled{}
	}
	return mailer.NewSMTP(mailer.SMTPOptions{
		Host: cfg.SMTP.Host, Port: cfg.SMTP.Port, Username: cfg.SMTP.Username, Password: cfg.SMTP.Password, From: cfg.SMTP.From,
		TLS: cfg.SMTP.TLS, Timeout: config.MailTimeout, Now: now,
	})
}
