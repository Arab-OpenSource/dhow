package session

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	ratelimit "github.com/Khaymah/dhow/apps/server/internal/Core/RateLimit"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Session/Gen"
)

type Deps struct {
	Service *Service
	Guard   *auth.Guard
	Limits  *ratelimit.PerClient
	Log     *slog.Logger
}

const (
	limitPassword = "password"
	limitMfa      = "mfa"
	limitOidc     = "oidc"
	limitPasskey  = "passkey"
)

func RateLimited() []ratelimit.Route {
	return []ratelimit.Route{
		{Method: http.MethodPost, Path: "/sessions", Group: limitPassword},
		{Method: http.MethodPost, Path: "/sessions/mfa", Group: limitMfa},
		{Method: http.MethodPost, Path: "/sessions/passkey", Group: limitPasskey},
		{Method: http.MethodPost, Path: "/oidc/authorize", Group: limitOidc},
		{Method: http.MethodPost, Path: "/oidc/callback", Group: limitOidc},
		{Method: http.MethodPost, Path: "/sessions/passkey-options", Group: limitPasskey},
	}
}

func Policies() []policy.Rule {
	return []policy.Rule{
		policy.Public(http.MethodPost, "/sessions"),
		policy.Authenticated(http.MethodGet, "/sessions", policy.SessionManageSelf),
		policy.Authenticated(http.MethodDelete, "/sessions/current", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/sessions/revoke-others", policy.SessionManageSelf),
		policy.Authenticated(http.MethodDelete, "/sessions/{id}", policy.SessionManageSelf),
		policy.Authenticated(http.MethodGet, "/me", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPut, "/me/password", policy.SessionManageSelf),
		policy.Public(http.MethodPost, "/sessions/mfa"),
		policy.Public(http.MethodPost, "/sessions/mfa/passkey-options"),
		policy.Public(http.MethodPost, "/sessions/passkey-options"),
		policy.Public(http.MethodPost, "/sessions/passkey"),
		policy.Authenticated(http.MethodGet, "/me/mfa", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/mfa/passkey-options", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/totp", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/totp/confirm", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/totp/disable", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/recovery-codes", policy.SessionManageSelf),
		policy.Authenticated(http.MethodGet, "/me/passkeys", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/passkeys", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/passkeys/options", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPatch, "/me/passkeys/{id}", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/passkeys/{id}/remove", policy.SessionManageSelf),
		policy.Public(http.MethodGet, "/oidc/providers"),
		policy.Public(http.MethodPost, "/oidc/authorize"),
		policy.Public(http.MethodPost, "/oidc/callback"),
		policy.Authenticated(http.MethodGet, "/me/identities", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/identities/authorize", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/identities/callback", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/identities/{id}/remove", policy.SessionManageSelf),
	}
}

func Routes(r chi.Router, d Deps) {
	strict := gen.NewStrictHandlerWithOptions(NewController(d.Service), nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  middleware.StrictRequestError,
		ResponseErrorHandlerFunc: middleware.StrictResponseError(d.Log),
	})
	guard := d.Guard.Middleware(policy.NewTable(Policies()...))
	gen.HandlerWithOptions(strict, gen.ChiServerOptions{
		BaseRouter:       r,
		Middlewares:      []gen.MiddlewareFunc{middleware.BodyLimit(config.AuthBodyMaxBytes), d.Limits.Middleware(RateLimited()...), guard},
		ErrorHandlerFunc: middleware.StrictRequestError,
	})
}
