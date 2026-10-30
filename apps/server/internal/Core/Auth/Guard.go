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

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	apperrors "github.com/Khaymah/dhow/apps/server/internal/Core/Errors"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	response "github.com/Khaymah/dhow/apps/server/internal/Core/Response"
	"github.com/Khaymah/dhow/internal/logging"
)

var (
	ErrNoSession = errors.New("no active session")
	ErrNoToken   = errors.New("no active api token")
)

type Resolved struct {
	SessionID     uuid.UUID
	UserID        uuid.UUID
	InstanceOwner bool
	CsrfHash      []byte
	ExpiresAt     time.Time
	Email         string
	Name          string
	Locale        string
	Memberships   []policy.Membership
}

type Resolver interface {
	Resolve(ctx context.Context, tokenHash []byte, now time.Time) (Resolved, error)
	Peek(ctx context.Context, tokenHash []byte, now time.Time) (Resolved, error)
}

type ResolvedToken struct {
	TokenID         uuid.UUID
	UserID          uuid.UUID
	OrganizationID  uuid.UUID
	Role            policy.Role
	Scopes          []policy.Action
	ExpiresAt       time.Time
	SecondFactorDue time.Time
	AdminDue        time.Time
	Email           string
	Name            string
	Locale          string
}

type MembershipToucher interface {
	TouchMembership(ctx context.Context, user, org uuid.UUID, now time.Time) error
}

type TokenResolver interface {
	ResolveToken(ctx context.Context, tokenHash []byte, now time.Time) (ResolvedToken, error)
}

type GuardOptions struct {
	Resolver  Resolver
	Members   MembershipToucher
	Tokens    TokenResolver
	Projects  ProjectResolver
	APITokens *APITokens
	CSRF      *CSRF
	PublicURL *url.URL
	Instance  func() policy.InstanceSettings
	Log       *slog.Logger
	Now       func() time.Time
}

type Guard struct {
	resolver  Resolver
	members   MembershipToucher
	tokens    TokenResolver
	projects  ProjectResolver
	apiTokens *APITokens
	csrf      *CSRF
	origin    string
	instance  func() policy.InstanceSettings
	log       *slog.Logger
	now       func() time.Time
}

func NewGuard(o GuardOptions) *Guard {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	instance := o.Instance
	if instance == nil {
		instance = func() policy.InstanceSettings { return policy.InstanceSettings{} }
	}
	return &Guard{
		resolver: o.Resolver, members: o.Members, tokens: o.Tokens, projects: o.Projects, apiTokens: o.APITokens, csrf: o.CSRF, origin: OriginOf(o.PublicURL),
		instance: instance, log: o.Log, now: now,
	}
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

func (g *Guard) Instance() policy.InstanceSettings { return g.instance() }

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
			if bearer, present := Authorization(r); present {
				var ae *apperrors.AppError
				if ctx, ae = g.authorizeToken(ctx, r, rule, bearer); ae != nil {
					if ae.Status >= http.StatusInternalServerError {
						g.log.ErrorContext(ctx, "api token authorization failed", "request_id", middleware.RequestIDFrom(ctx), "error", ae)
					}
					response.Error(w, r, ae)
					return
				}
				next.ServeHTTP(w, r.WithContext(ctx))
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
	if c, err := r.Cookie(config.MfaCookieName); err == nil {
		meta.MfaToken = c.Value
	}
	if c, err := r.Cookie(config.OidcCookieName); err == nil {
		meta.OidcToken = c.Value
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
	if rule.Scope == policy.ScopeInstance {
		if !policy.CanInstance(principal.Subject, rule.Action, g.instance()) {
			return ctx, apperrors.Forbidden()
		}
		return WithPrincipal(ctx, principal), nil
	}
	var resource policy.Resource
	if scopedToOrganization(rule) {
		id, err := uuid.Parse(chi.URLParam(r, rule.OrgParam))
		if err != nil {
			return ctx, apperrors.NotFound()
		}
		if _, member := principal.RoleIn(id); !member {
			return ctx, apperrors.NotFound()
		}
		resource.OrganizationID = id
	}
	resource, ae = g.projectOf(ctx, r, rule, resource, principal.UserID)
	if ae != nil {
		return ctx, ae
	}
	if ae := g.permit(principal.Subject, rule.Action, resource); ae != nil {
		return ctx, ae
	}
	if resource.OrganizationID != uuid.Nil {
		g.touchMembership(ctx, principal.Subject, resource.OrganizationID)
	}
	return WithPrincipal(ctx, withProjectRole(principal, resource)), nil
}

func (g *Guard) touchMembership(ctx context.Context, s policy.Subject, org uuid.UUID) {
	if g.members == nil {
		return
	}
	now := g.now()
	for _, m := range s.Memberships {
		if m.OrganizationID != org || now.Sub(m.LastSeenAt) < config.MembershipTouchInterval {
			continue
		}
		if err := g.members.TouchMembership(ctx, s.UserID, org, now); err != nil {
			g.log.WarnContext(ctx, "touch membership failed", "error", err)
		}
		return
	}
}

func (g *Guard) permit(s policy.Subject, action policy.Action, resource policy.Resource) *apperrors.AppError {
	if !policy.Can(s, action, resource) {
		return apperrors.Forbidden()
	}
	if policy.SecondFactorBlocks(s, action, resource, g.now()) {
		return apperrors.SecondFactorRequired()
	}
	return nil
}

func (g *Guard) Reauthorize(ctx context.Context, action policy.Action, org uuid.UUID) error {
	principal, _, ae := g.authenticateWith(ctx, g.resolver.Peek)
	if ae != nil {
		return ae
	}
	if _, member := principal.RoleIn(org); !member {
		return apperrors.NotFound()
	}
	if ae := g.permit(principal.Subject, action, policy.Resource{OrganizationID: org}); ae != nil {
		return ae
	}
	return nil
}

func (g *Guard) organizationOf(r *http.Request, rule policy.Rule) (policy.Resource, *apperrors.AppError) {
	if !scopedToOrganization(rule) {
		return policy.Resource{}, nil
	}
	id, err := uuid.Parse(chi.URLParam(r, rule.OrgParam))
	if err != nil {
		return policy.Resource{}, apperrors.NotFound()
	}
	return policy.Resource{OrganizationID: id}, nil
}

func (g *Guard) authorizeToken(ctx context.Context, r *http.Request, rule policy.Rule, bearer string) (context.Context, *apperrors.AppError) {
	meta := RequestMetaFrom(ctx)
	if meta.SessionToken != "" || meta.MfaToken != "" || meta.OidcToken != "" || g.apiTokens == nil || g.tokens == nil {
		return ctx, apperrors.Unauthenticated()
	}
	hash, ok := g.apiTokens.Hash(bearer)
	if !ok {
		return ctx, apperrors.Unauthenticated()
	}
	if !rule.Tokens {
		return ctx, apperrors.SessionRequired()
	}
	res, err := g.tokens.ResolveToken(ctx, hash, g.now())
	switch {
	case errors.Is(err, ErrNoToken):
		return ctx, apperrors.Unauthenticated()
	case err != nil:
		return ctx, apperrors.Internal(err)
	}
	resource, ae := g.organizationOf(r, rule)
	if ae != nil {
		return ctx, ae
	}
	if resource.OrganizationID != res.OrganizationID {
		return ctx, apperrors.NotFound()
	}
	resource, ae = g.projectOf(ctx, r, rule, resource, res.UserID)
	if ae != nil {
		return ctx, ae
	}
	principal := Principal{
		Subject: policy.Subject{
			UserID: res.UserID, TokenID: res.TokenID, TokenScopes: res.Scopes,
			Memberships: []policy.Membership{{
				OrganizationID: res.OrganizationID, Role: res.Role, SecondFactorDue: res.SecondFactorDue, AdminSecondFactorDue: res.AdminDue,
			}},
		},
		Email: res.Email, Name: res.Name, Locale: res.Locale, ExpiresAt: res.ExpiresAt,
	}
	if ae := g.permit(principal.Subject, rule.Action, resource); ae != nil {
		return ctx, ae
	}
	return WithPrincipal(ctx, withProjectRole(principal, resource)), nil
}

func (g *Guard) authenticate(ctx context.Context) (Principal, []byte, *apperrors.AppError) {
	return g.authenticateWith(ctx, g.resolver.Resolve)
}

func (g *Guard) authenticateWith(ctx context.Context, resolve func(context.Context, []byte, time.Time) (Resolved, error)) (Principal, []byte, *apperrors.AppError) {
	token := RequestMetaFrom(ctx).SessionToken
	hash, ok := HashSessionToken(token, config.SessionTokenBytes)
	if !ok {
		return Principal{}, nil, apperrors.Unauthenticated()
	}
	res, err := resolve(ctx, hash, g.now())
	switch {
	case errors.Is(err, ErrNoSession):
		return Principal{}, nil, apperrors.Unauthenticated()
	case err != nil:
		return Principal{}, nil, apperrors.Internal(err)
	}
	return Principal{
		Subject: policy.Subject{
			UserID: res.UserID, SessionID: res.SessionID, InstanceOwner: res.InstanceOwner, Memberships: res.Memberships,
		},
		Email:     res.Email,
		Name:      res.Name,
		Locale:    res.Locale,
		CSRFToken: g.csrf.Token(token),
		ExpiresAt: res.ExpiresAt,
	}, res.CsrfHash, nil
}
