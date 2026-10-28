package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
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
	jobs "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs"
	runners "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Runners"
	session "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session"
	system "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"
	systemmodels "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System/Models"
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

	LogStreamWriteWait time.Duration
	LogReadWait        time.Duration
}

type Wired struct {
	Options       Options
	Handler       http.Handler
	RunnerHandler http.Handler
	Identity      *identity.Service
	Jobs          *jobs.Service
	LogPool       *pgxpool.Pool
}

func (w *Wired) Close() {
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
	logPool, err := logReadPool(ctx, in.Pool, in.Config.Logs)
	if err != nil {
		return nil, err
	}
	jobsOptions := jobs.Options{
		DB: in.Pool, Keys: runnerKeys, Secrets: in.Secrets, Bus: bus, Listen: listener(in.Pool),
		MaxPolls: in.Config.RunnerMaxPolls, Logs: in.Config.Logs, StreamWriteWait: in.LogStreamWriteWait,
		ReadWait: in.LogReadWait, Log: in.Log,
	}
	if logPool != nil {
		jobsOptions.Reads = logPool
	}
	jobsService := jobs.NewService(jobsOptions)
	jobs.Subscribe(bus, jobsService)

	opts := Options{
		Config: in.Config,
		Log:    in.Log,
		System: system.Deps{
			DB: in.Pool, Migrations: in.Migrations, Queries: sqlc.New(in.Pool), Build: in.Build, Log: in.Log,
			Checks: []system.ReadinessCheck{{Name: systemmodels.CheckLogPartitions, Run: jobsService.CheckLogPartition}},
		},
		Identity: identityService,
		Session:  sessionService,
		Audit:    auditService,
		Runners:  runnersService,
		Jobs:     jobsService,
		Gate:     gate,
		CSRF:     csrf,
		Now:      in.Now,
		Web:      in.Web,
	}
	wired := &Wired{Options: opts, Handler: NewRouter(opts), Identity: identityService, Jobs: jobsService, LogPool: logPool}
	if in.Config.RunnerPlaneSeparate() {
		wired.RunnerHandler = RunnerPlaneServer(opts)
	}
	return wired, nil
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
