package system

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System/Gen"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

type Deps struct {
	DB         Pinger
	Migrations MigrationStatus
	Queries    InstanceQuerier
	Build      buildinfo.Info
	Log        *slog.Logger
	Guard      *auth.Guard
	Checks     []ReadinessCheck
}

func Policies() []policy.Rule {
	return []policy.Rule{
		policy.Public(http.MethodGet, "/health/live"),
		policy.Public(http.MethodGet, "/health/ready"),
		policy.Public(http.MethodGet, "/meta"),
	}
}

func Routes(r chi.Router, d Deps) {
	controller := NewController(
		NewHealthService(d.DB, d.Migrations, d.Log, d.Checks...),
		NewMetaService(d.Queries, d.Build),
	)
	strict := gen.NewStrictHandlerWithOptions(controller, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  middleware.StrictRequestError,
		ResponseErrorHandlerFunc: middleware.StrictResponseError(d.Log),
	})
	guard := d.Guard.Middleware(policy.NewTable(Policies()...))
	gen.HandlerWithOptions(strict, gen.ChiServerOptions{
		BaseRouter:       r,
		Middlewares:      []gen.MiddlewareFunc{guard},
		ErrorHandlerFunc: middleware.StrictRequestError,
	})
}
