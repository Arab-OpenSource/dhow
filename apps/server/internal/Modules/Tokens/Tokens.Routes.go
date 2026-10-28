package tokens

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Tokens/Gen"
)

type Deps struct {
	Service *Service
	Guard   *auth.Guard
	Log     *slog.Logger
}

const orgTokensPath = "/organizations/{id}/api-tokens"

func Policies() []policy.Rule {
	return []policy.Rule{
		policy.Organization(http.MethodPost, orgTokensPath, policy.TokenCreate, "id"),
		policy.Organization(http.MethodGet, orgTokensPath, policy.TokenManage, "id"),
		policy.Organization(http.MethodPost, orgTokensPath+"/{tokenId}/revoke", policy.TokenManage, "id"),
		policy.Authenticated(http.MethodGet, "/me/api-tokens", policy.SessionManageSelf),
		policy.Authenticated(http.MethodPost, "/me/api-tokens/{id}/revoke", policy.SessionManageSelf),
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
