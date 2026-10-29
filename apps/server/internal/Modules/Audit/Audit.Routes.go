package audit

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	eventbus "github.com/Khaymah/dhow/apps/server/internal/Core/EventBus"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Audit/Gen"
)

type Deps struct {
	Service *Service
	Guard   *auth.Guard
	Log     *slog.Logger
}

func Policies() []policy.Rule {
	return []policy.Rule{
		policy.Organization(http.MethodGet, "/organizations/{id}/audit-events", policy.AuditRead, "id").AllowTokens(),
		policy.Organization(http.MethodGet, "/organizations/{id}/audit-events/verify", policy.AuditRead, "id").AllowTokens(),
		policy.Organization(http.MethodGet, "/organizations/{id}/audit-events/{eventId}", policy.AuditRead, "id").AllowTokens(),
	}
}

func Subscribe(bus *eventbus.Bus, s *Service) {
	for _, name := range eventbus.AuditedEvents() {
		bus.Subscribe(name, s.Handle)
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
		Middlewares:      []gen.MiddlewareFunc{guard},
		ErrorHandlerFunc: middleware.StrictRequestError,
	})
}
