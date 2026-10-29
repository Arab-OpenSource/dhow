package app

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	appcrypto "github.com/Khaymah/dhow/apps/server/internal/Core/Crypto"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	mailer "github.com/Khaymah/dhow/apps/server/internal/Core/Mail"
	oidcauth "github.com/Khaymah/dhow/apps/server/internal/Core/Oidc"
	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
	runnerauth "github.com/Khaymah/dhow/apps/server/internal/Core/RunnerAuth"
	instancesettings "github.com/Khaymah/dhow/apps/server/internal/Core/Settings"
	retention "github.com/Khaymah/dhow/apps/server/internal/Database/Retention"
	sqlc "github.com/Khaymah/dhow/apps/server/internal/Database/Sqlc"
	alerts "github.com/Khaymah/dhow/apps/server/internal/Modules/Alerts"
	audit "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit"
	identity "github.com/Khaymah/dhow/apps/server/internal/Modules/Identity"
	jobs "github.com/Khaymah/dhow/apps/server/internal/Modules/Jobs"
	overview "github.com/Khaymah/dhow/apps/server/internal/Modules/Overview"
	preferences "github.com/Khaymah/dhow/apps/server/internal/Modules/Preferences"
	runners "github.com/Khaymah/dhow/apps/server/internal/Modules/Runners"
	session "github.com/Khaymah/dhow/apps/server/internal/Modules/Session"
	settings "github.com/Khaymah/dhow/apps/server/internal/Modules/Settings"
	system "github.com/Khaymah/dhow/apps/server/internal/Modules/System"
	systemmodels "github.com/Khaymah/dhow/apps/server/internal/Modules/System/Models"
	tokens "github.com/Khaymah/dhow/apps/server/internal/Modules/Tokens"
	"github.com/Khaymah/dhow/internal/buildinfo"
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
	MailTLS    *tls.Config

	LogStreamWriteWait time.Duration
	LogReadWait        time.Duration
	OidcTransport      http.RoundTripper
}

type Wired struct {
	Options       Options
	Handler       http.Handler
	RunnerHandler http.Handler
	Identity      *identity.Service
	Jobs          *jobs.Service
	LogPool       *pgxpool.Pool
	Retention     *retention.Runner
	Alerts        *alerts.Service
	Settings      *instancesettings.Store
}

func (w *Wired) Close() {
	if w.Alerts != nil {
		w.Alerts.Close()
	}
	if w.LogPool != nil {
		w.LogPool.Close()
	}
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
	store, err := settingsStore(ctx, in)
	if err != nil {
		return nil, err
	}
	passwords := passwordsFor(in, store)
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
	limits := func() config.RateLimits { return store.Current().Values.RateLimits }
	limiter := ratelimit.NewPostgres(in.Pool, in.Now)

	auditService := audit.NewService(in.Pool, in.Log, in.Now)
	audit.Subscribe(bus, auditService)
	sessionService := session.NewService(session.Options{
		DB: in.Pool, Hasher: hasher, Passwords: passwords, CSRF: csrf, Sealer: sealer, Recovery: recovery, WebAuthn: passkeys,
		Bus: bus, Instance: store.Instance, Log: in.Log, Now: in.Now,
		Oidc: external.providers, OidcKey: external.oidcKey, CallbackBase: in.Config.PublicURL.String() + config.OidcCallbackPath,
	})
	mail := mailerFor(in, store)
	alertsService := alertsFor(in, mail, bus)
	identityService := identity.NewService(identity.Options{
		DB: in.Pool, Hasher: hasher, Passwords: passwords, Bus: bus, Factors: sessionService, Mailer: mail,
		PublicURL: in.Config.PublicURL.String(), Limiter: limiter, Limits: limits, Log: in.Log, Now: in.Now,
	})

	runnersService := runners.NewService(runners.Options{
		DB: in.Pool, Keys: runnerKeys, Bus: bus, Origin: auth.OriginOf(in.Config.PublicURL), PublicURL: in.Config.PublicURL.String(),
		Dev: in.Config.Dev, Log: in.Log, Now: in.Now,
	})
	tokensService := tokens.NewService(tokens.Options{
		DB: in.Pool, Tokens: external.apiTokens, Factors: sessionService, Bus: bus, Log: in.Log, Now: in.Now,
	})
	gate := runnerauth.NewGate(runnerauth.GateOptions{Verifier: runnersService, Log: in.Log, Now: in.Now})
	jobsService, logPool, err := buildJobs(ctx, in, runnerKeys, bus, store)
	if err != nil {
		return nil, err
	}

	opts := Options{
		Config: in.Config,
		Log:    in.Log,
		System: system.Deps{
			DB: in.Pool, Migrations: in.Migrations, Queries: sqlc.New(in.Pool), Build: in.Build, Log: in.Log,
			Checks: []system.ReadinessCheck{{Name: systemmodels.CheckLogPartitions, Run: jobsService.CheckLogPartition}},
		},
		Identity:  identityService,
		Session:   sessionService,
		Audit:     auditService,
		Runners:   runnersService,
		Jobs:      jobsService,
		Overview:  overview.NewService(overview.Options{Counter: sqlc.New(in.Pool), Log: in.Log, Now: in.Now}),
		Tokens:    tokensService,
		Prefs:     preferences.NewService(in.Pool, in.Log, in.Now),
		Gate:      gate,
		CSRF:      csrf,
		APITokens: external.apiTokens,
		Limits:    ratelimit.NewLivePerClient(limiter, func() int { return limits().Public }, config.RateLimitPublicWindow, in.Log),
		Settings: settings.NewService(settings.Options{
			Store: store, DB: in.Pool, Factors: sessionService, Bus: bus, Mailer: mail, Limiter: limiter, Log: in.Log, Now: in.Now,
		}),
		Instance: store.Instance,
		Now:      in.Now,
		Web:      in.Web,
	}
	wired := &Wired{
		Options: opts, Handler: NewRouter(opts), Identity: identityService, Jobs: jobsService, LogPool: logPool, Settings: store,
		Retention: retention.New(retention.Options{Pool: in.Pool, Log: in.Log, Now: in.Now}),
		Alerts:    alertsService,
	}
	if in.Config.RunnerPlaneSeparate() {
		wired.RunnerHandler = RunnerPlaneServer(opts)
	}
	return wired, nil
}

func buildJobs(
	ctx context.Context, in Infra, keys *runnerauth.Keys, bus *eventbus.Bus, store *instancesettings.Store,
) (*jobs.Service, *pgxpool.Pool, error) {
	logPool, err := logReadPool(ctx, in.Pool, in.Config.Logs)
	if err != nil {
		return nil, nil, err
	}
	opts := jobs.Options{
		DB: in.Pool, Keys: keys, Secrets: in.Secrets, Bus: bus, Listen: listener(in.Pool),
		MaxPolls: in.Config.RunnerMaxPolls, Logs: in.Config.Logs, StreamWriteWait: in.LogStreamWriteWait,
		ReadWait: in.LogReadWait, Log: in.Log,
		LogSettings: func() config.LogSettings { return store.Current().Values.Logs },
	}
	if logPool != nil {
		opts.Reads = logPool
	}
	service := jobs.NewService(opts)
	jobs.Subscribe(bus, service)
	return service, logPool, nil
}

func logReadPool(ctx context.Context, pool *pgxpool.Pool, logs config.Logs) (*pgxpool.Pool, error) {
	if pool == nil {
		return nil, nil
	}
	conns := logs.ReadConns
	if conns <= 0 {
		conns = config.LogReadConnsDefault
	}
	cfg := pool.Config().Copy()
	cfg.MaxConns = int32(conns)
	cfg.MinConns = 0
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(config.LogReadStatementWait.Milliseconds(), 10)
	cfg.ConnConfig.RuntimeParams["application_name"] = config.DatabaseAppName + "-logs"
	logPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open the job log read pool: %w", err)
	}
	return logPool, nil
}

type externalDeps struct {
	providers *oidcauth.Registry
	oidcKey   []byte
	apiTokens *auth.APITokens
}

func externalAuth(in Infra) (externalDeps, error) {
	providers, err := oidcauth.NewRegistry(in.Config.OIDCProviders, oidcauth.Options{Transport: in.OidcTransport, Now: in.Now, AllowHTTP: in.Config.Dev})
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

func alertsFor(in Infra, mail mailer.Sender, bus *eventbus.Bus) *alerts.Service {
	service := alerts.NewService(alerts.Options{
		DB: in.Pool, Mailer: mail, PublicURL: in.Config.PublicURL.String(), Log: in.Log, Now: in.Now,
	})
	alerts.Subscribe(bus, service)
	return service
}

func mailerFor(in Infra, store *instancesettings.Store) mailer.Sender {
	if in.Mailer != nil {
		return in.Mailer
	}
	return instancesettings.NewMailer(store, in.MailTLS, in.Now)
}

func passwordsFor(in Infra, store *instancesettings.Store) *auth.PasswordPolicy {
	var breach auth.BreachChecker = auth.NewHIBP(config.BreachedRangeURL, config.BreachedCheckTimeout, nil)
	if in.Breach != nil {
		breach = in.Breach
	}
	return auth.NewPasswordPolicy(instancesettings.NewBreach(store, breach), in.Log, config.PasswordMinLength, config.PasswordMaxLength)
}

func settingsStore(ctx context.Context, in Infra) (*instancesettings.Store, error) {
	opts := instancesettings.Options{SecretKey: in.Config.SecretKey, Env: in.Config.Settings, Log: in.Log, Now: in.Now}
	if in.Pool != nil {
		opts.DB = in.Pool
		opts.Listen = instancesettings.Listener(listener(in.Pool))
	}
	store, err := instancesettings.New(opts)
	if err != nil {
		return nil, fmt.Errorf("invalid instance settings:\n%w", err)
	}
	if err := store.Load(ctx); err != nil {
		return nil, fmt.Errorf("invalid instance settings:\n%w", err)
	}
	return store, nil
}
