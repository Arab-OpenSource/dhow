package app

import (
	"context"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	appcrypto "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Crypto"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	mailer "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Mail"
	oidcauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Oidc"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	runnerauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/RunnerAuth"
	sqlc "github.com/Arab-OpenSource/dhow/apps/server/internal/Database/Sqlc"
	audit "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Audit"
	identity "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity"
	jobs "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs"
	preferences "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Preferences"
	runners "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Runners"
	session "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session"
	system "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"
	tokens "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Tokens"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

type Infra struct {
	Config     config.Config
	Log        *slog.Logger
	Pool       *pgxpool.Pool
	Migrations system.MigrationStatus
	Build      buildinfo.Info
	Breach     auth.BreachChecker
	Secrets    jobs.SecretResolver
	HashParams *auth.Params
	Now        func() time.Time
	Web        http.Handler
	Mailer     mailer.Sender

	OidcTransport http.RoundTripper
}

type Wired struct {
	Options       Options
	Handler       http.Handler
	RunnerHandler http.Handler
	Identity      *identity.Service
	Jobs          *jobs.Service
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
	external, err := externalAuth(in)
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
		Oidc: external.providers, OidcKey: external.oidcKey, CallbackBase: in.Config.PublicURL.String() + config.OidcCallbackPath,
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
	tokensService := tokens.NewService(tokens.Options{
		DB: in.Pool, Tokens: external.apiTokens, Factors: sessionService, Bus: bus, Log: in.Log, Now: in.Now,
	})
	gate := runnerauth.NewGate(runnerauth.GateOptions{Verifier: runnersService, Log: in.Log, Now: in.Now})
	jobsService := jobs.NewService(jobs.Options{
		DB: in.Pool, Keys: runnerKeys, Secrets: in.Secrets, Bus: bus, Listen: listener(in.Pool),
		MaxPolls: in.Config.RunnerMaxPolls, Log: in.Log,
	})
	jobs.Subscribe(bus, jobsService)

	opts := Options{
		Config: in.Config,
		Log:    in.Log,
		System: system.Deps{
			DB: in.Pool, Migrations: in.Migrations, Queries: sqlc.New(in.Pool), Build: in.Build, Log: in.Log,
		},
		Identity:  identityService,
		Session:   sessionService,
		Audit:     auditService,
		Runners:   runnersService,
		Jobs:      jobsService,
		Tokens:    tokensService,
		Prefs:     preferences.NewService(in.Pool, in.Log, in.Now),
		Gate:      gate,
		CSRF:      csrf,
		APITokens: external.apiTokens,
		Now:       in.Now,
		Web:       in.Web,
	}
	wired := &Wired{Options: opts, Handler: NewRouter(opts), Identity: identityService, Jobs: jobsService}
	if in.Config.RunnerPlaneSeparate() {
		wired.RunnerHandler = RunnerPlaneServer(opts)
	}
	return wired, nil
}

type externalDeps struct {
	providers *oidcauth.Registry
	oidcKey   []byte
	apiTokens *auth.APITokens
}

func externalAuth(in Infra) (externalDeps, error) {
	providers, err := oidcauth.NewRegistry(in.Config.OIDCProviders, oidcauth.Options{Transport: in.OidcTransport, Now: in.Now})
	if err != nil {
		return externalDeps{}, err
	}
	oidcKey, err := appcrypto.DeriveKey(in.Config.SecretKey, config.OidcFlowKeyLabel)
	if err != nil {
		return externalDeps{}, err
	}
	apiTokens, err := auth.NewAPITokens(in.Config.SecretKey)
	if err != nil {
		return externalDeps{}, err
	}
	return externalDeps{providers: providers, oidcKey: oidcKey, apiTokens: apiTokens}, nil
}

func listener(pool *pgxpool.Pool) jobs.Listener {
	if pool == nil {
		return nil
	}
	return func(ctx context.Context) (*pgx.Conn, error) {
		return pgx.ConnectConfig(ctx, pool.Config().ConnConfig.Copy())
	}
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
