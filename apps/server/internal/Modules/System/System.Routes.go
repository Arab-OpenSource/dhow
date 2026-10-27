package system

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System/Gen"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

type Deps struct {
	DB         Pinger
	Migrations MigrationStatus
	Queries    InstanceQuerier
	Build      buildinfo.Info
	Log        *slog.Logger
}

func Routes(r chi.Router, d Deps) {
	controller := NewController(
		NewHealthService(d.DB, d.Migrations, d.Log),
		NewMetaService(d.Queries, d.Build),
	)
	strict := gen.NewStrictHandlerWithOptions(controller, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  middleware.StrictRequestError,
		ResponseErrorHandlerFunc: middleware.StrictResponseError(d.Log),
	})
	gen.HandlerWithOptions(strict, gen.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: middleware.StrictRequestError,
	})
}
