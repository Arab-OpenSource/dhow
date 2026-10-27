package session

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session/Gen"
)

type Deps struct {
	Service *Service
	Guard   *auth.Guard
	Log     *slog.Logger
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
		Middlewares:      []gen.MiddlewareFunc{middleware.BodyLimit(config.AuthBodyMaxBytes), guard},
		ErrorHandlerFunc: middleware.StrictRequestError,
	})
}
