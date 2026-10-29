package ratelimit

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	response "github.com/Khaymah/dhow/apps/server/internal/Core/Response"
	"github.com/Khaymah/dhow/internal/logging"
)

const clientPrefix = "ip-"

type Route struct {
	Method string
	Path   string
	Group  string
}

func (r Route) Key() string { return r.Method + " " + r.Path }

type PerClient struct {
	limiter Limiter
	max     func() int
	window  time.Duration
	log     *slog.Logger
}

func NewPerClient(limiter Limiter, maxPerWindow int, window time.Duration, log *slog.Logger) *PerClient {
	return NewLivePerClient(limiter, func() int { return maxPerWindow }, window, log)
}

func NewLivePerClient(limiter Limiter, maxPerWindow func() int, window time.Duration, log *slog.Logger) *PerClient {
	return &PerClient{limiter: limiter, max: maxPerWindow, window: window, log: log}
}

func (p *PerClient) LimitFor(r Route) Limit {
	return Limit{Name: clientPrefix + r.Group, Max: p.max(), Window: p.window}
}

func (p *PerClient) Middleware(routes ...Route) func(http.Handler) http.Handler {
	table := make(map[string]Route, len(routes))
	for _, r := range routes {
		table[r.Key()] = r
	}
	return func(next http.Handler) http.Handler {
		if p == nil || len(table) == 0 {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			route, ok := table[r.Method+" "+strings.TrimPrefix(chi.RouteContext(ctx).RoutePattern(), config.APIPrefix)]
			if !ok {
				next.ServeHTTP(w, r)
				return
			}
			network := Network(middleware.ClientIPFrom(ctx))
			d, err := p.limiter.Take(ctx, p.LimitFor(route), network)
			if err != nil {
				p.log.ErrorContext(ctx, "rate limit check failed", "group", route.Group, "request_id", middleware.RequestIDFrom(ctx), "error", err)
				response.Error(w, r, apperrors.Internal(err))
				return
			}
			if !d.Allowed {
				p.log.DebugContext(ctx, "request rate limited", "group", route.Group, "network", network,
					"path", logging.UserValue(r.URL.Path), "retry_after", d.RetryAfter.String())
				response.Error(w, r, apperrors.RateLimited(d.RetryAfter))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
