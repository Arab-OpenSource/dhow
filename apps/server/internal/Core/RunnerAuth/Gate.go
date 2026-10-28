package runnerauth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"connectrpc.com/connect"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	"github.com/Khaymah/dhow/internal/logging"
)

type GateOptions struct {
	Verifier Verifier
	Log      *slog.Logger
	Now      func() time.Time
}

type Gate struct {
	verifier Verifier
	log      *slog.Logger
	now      func() time.Time
}

func NewGate(o GateOptions) *Gate {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Gate{verifier: o.Verifier, log: o.Log, now: now}
}

func (g *Gate) RequestGate(table *policy.Table) connect.HandlerOption {
	return connect.WithRequestGate(func(ctx context.Context, spec connect.Spec, _ connect.Peer, header http.Header) (context.Context, error) {
		return g.check(ctx, table, spec.Procedure, header)
	})
}

func (g *Gate) check(ctx context.Context, table *policy.Table, procedure string, header http.Header) (context.Context, error) {
	ctx = WithRequestMeta(ctx, RequestMeta{
		IP: middleware.ClientIPFrom(ctx), RunnerVersion: logging.UserValue(header.Get(config.RunnerVersionHeader)),
	})
	rule, ok := table.Lookup(http.MethodPost, procedure)
	if !ok || rule.Scope != policy.ScopeRunner {
		g.log.ErrorContext(ctx, "runner procedure has no valid policy declaration",
			"request_id", middleware.RequestIDFrom(ctx), "procedure", logging.UserValue(procedure))
		return ctx, PermissionDenied()
	}
	if header.Get(config.RunnerProtocolHeader) != config.RunnerProtocolVersion {
		return ctx, UpgradeRequired()
	}
	if carriesBrowserSession(header) {
		return ctx, Unauthenticated()
	}
	if rule.Runner != policy.RunnerSession {
		return ctx, nil
	}
	hash, ok := HashAccessToken(BearerToken(header))
	if !ok {
		return ctx, Unauthenticated()
	}
	p, err := g.verifier.Resolve(ctx, hash, g.now())
	switch {
	case errors.Is(err, ErrRevoked):
		return ctx, RunnerRevoked()
	case errors.Is(err, ErrNoSession):
		return ctx, Unauthenticated()
	case err != nil:
		g.log.ErrorContext(ctx, "resolve runner session", "request_id", middleware.RequestIDFrom(ctx), "error", err)
		return ctx, Internal()
	}
	return WithPrincipal(ctx, p), nil
}

func carriesBrowserSession(header http.Header) bool {
	r := http.Request{Header: header}
	for _, name := range []string{config.SessionCookieName, config.MfaCookieName} {
		if _, err := r.Cookie(name); err == nil {
			return true
		}
	}
	return false
}
