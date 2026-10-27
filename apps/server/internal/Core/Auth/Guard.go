package auth

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	response "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Response"
	"github.com/Arab-OpenSource/dhow/internal/logging"
)

var ErrNoSession = errors.New("no active session")

type Resolved struct {
	SessionID   uuid.UUID
	UserID      uuid.UUID
	CsrfHash    []byte
	ExpiresAt   time.Time
	Email       string
	Name        string
	Locale      string
	Memberships []policy.Membership
}

type Resolver interface {
	Resolve(ctx context.Context, tokenHash []byte, now time.Time) (Resolved, error)
}

type GuardOptions struct {
	Resolver  Resolver
	CSRF      *CSRF
	PublicURL *url.URL
	Log       *slog.Logger
	Now       func() time.Time
}

type Guard struct {
	resolver Resolver
	csrf     *CSRF
	origin   string
	log      *slog.Logger
	now      func() time.Time
}

func NewGuard(o GuardOptions) *Guard {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	return &Guard{resolver: o.Resolver, csrf: o.CSRF, origin: OriginOf(o.PublicURL), log: o.Log, now: now}
}

func OriginOf(u *url.URL) string {
	host := strings.ToLower(u.Host)
	switch u.Scheme {
	case "https":
		host = strings.TrimSuffix(host, ":443")
	case "http":
		host = strings.TrimSuffix(host, ":80")
	}
	return strings.ToLower(u.Scheme) + "://" + host
}

func (g *Guard) CSRF() *CSRF { return g.csrf }

func (g *Guard) Middleware(table *policy.Table) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := WithRequestMeta(r.Context(), g.requestMeta(r))
			rule, ok := table.Lookup(r.Method, strings.TrimPrefix(chi.RouteContext(ctx).RoutePattern(), config.APIPrefix))
			if !ok {
				g.log.ErrorContext(ctx, "route has no valid policy declaration", "method", r.Method, "path", logging.UserValue(r.URL.Path))
				response.Error(w, r, apperrors.Forbidden())
				return
			}
			if rule.Unsafe() && !g.originMatches(r) {
				response.Error(w, r, apperrors.CSRFFailed())
				return
			}
			if rule.Scope != policy.ScopePublic {
				var ae *apperrors.AppError
				if ctx, ae = g.authorize(ctx, r, rule); ae != nil {
					if ae.Status >= http.StatusInternalServerError {
						g.log.ErrorContext(ctx, "authorization failed", "request_id", middleware.RequestIDFrom(ctx), "error", ae)
					}
					response.Error(w, r, ae)
					return
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (g *Guard) requestMeta(r *http.Request) RequestMeta {
	meta := RequestMeta{IP: middleware.ClientIPFrom(r.Context()), UserAgent: logging.UserValue(r.UserAgent())}
	if c, err := r.Cookie(config.SessionCookieName); err == nil {
		meta.SessionToken = c.Value
	}
	return meta
}

func (g *Guard) originMatches(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin != "" && strings.EqualFold(origin, g.origin)
}

func (g *Guard) authorize(ctx context.Context, r *http.Request, rule policy.Rule) (context.Context, *apperrors.AppError) {
	principal, csrfHash, ae := g.authenticate(ctx)
	if ae != nil {
		return ctx, ae
	}
	if rule.Unsafe() && !g.csrf.Verify(r.Header.Get(config.CSRFHeaderName), csrfHash) {
		return ctx, apperrors.CSRFFailed()
	}
	var resource policy.Resource
	if rule.Scope == policy.ScopeOrganization {
		id, err := uuid.Parse(chi.URLParam(r, rule.OrgParam))
		if err != nil {
			return ctx, apperrors.NotFound()
		}
		if _, member := principal.RoleIn(id); !member {
			return ctx, apperrors.NotFound()
		}
		resource.OrganizationID = id
	}
	if !policy.Can(principal.Subject, rule.Action, resource) {
		return ctx, apperrors.Forbidden()
	}
	return WithPrincipal(ctx, principal), nil
}

func (g *Guard) authenticate(ctx context.Context) (Principal, []byte, *apperrors.AppError) {
	token := RequestMetaFrom(ctx).SessionToken
	hash, ok := HashSessionToken(token, config.SessionTokenBytes)
	if !ok {
		return Principal{}, nil, apperrors.Unauthenticated()
	}
	res, err := g.resolver.Resolve(ctx, hash, g.now())
	switch {
	case errors.Is(err, ErrNoSession):
		return Principal{}, nil, apperrors.Unauthenticated()
	case err != nil:
		return Principal{}, nil, apperrors.Internal(err)
	}
	return Principal{
		Subject:   policy.Subject{UserID: res.UserID, SessionID: res.SessionID, Memberships: res.Memberships},
		Email:     res.Email,
		Name:      res.Name,
		Locale:    res.Locale,
		CSRFToken: g.csrf.Token(token),
		ExpiresAt: res.ExpiresAt,
	}, res.CsrfHash, nil
}
