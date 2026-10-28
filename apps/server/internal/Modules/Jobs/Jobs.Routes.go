package jobs

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	eventbus "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/EventBus"
	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	runnerauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/RunnerAuth"
	gen "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs/Gen"
	"github.com/Arab-OpenSource/dhow/packages/proto/gen/go/dhow/runner/v1/runnerv1connect"
)

type Deps struct {
	Service *Service
	Guard   *auth.Guard
	Gate    *runnerauth.Gate
	Log     *slog.Logger
}

func Policies() []policy.Rule {
	return []policy.Rule{
		policy.Organization(http.MethodGet, "/organizations/{id}/jobs", policy.JobRead, "id"),
		policy.Organization(http.MethodPost, "/organizations/{id}/jobs", policy.JobCreate, "id"),
		policy.Organization(http.MethodGet, "/organizations/{id}/jobs/{jobId}", policy.JobRead, "id"),
		policy.Organization(http.MethodPost, "/organizations/{id}/jobs/{jobId}/cancel", policy.JobCancel, "id"),
	}
}

func Procedures() []policy.Rule {
	return []policy.Rule{
		policy.Runner(runnerv1connect.LeaseServiceLeaseProcedure, policy.RunnerSession),
		policy.Runner(runnerv1connect.LeaseServiceStartProcedure, policy.RunnerSession),
		policy.Runner(runnerv1connect.LeaseServiceHeartbeatProcedure, policy.RunnerSession),
		policy.Runner(runnerv1connect.LeaseServiceBeginStepProcedure, policy.RunnerSession),
		policy.Runner(runnerv1connect.LeaseServiceEndStepProcedure, policy.RunnerSession),
		policy.Runner(runnerv1connect.LeaseServiceCompleteProcedure, policy.RunnerSession),
	}
}

func Subscribe(bus *eventbus.Bus, s *Service) {
	bus.Subscribe(eventbus.RunnerRevoked, s.HandleRunnerRevoked)
}

func Routes(r chi.Router, d Deps) {
	strict := gen.NewStrictHandlerWithOptions(NewController(d.Service), nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  middleware.StrictRequestError,
		ResponseErrorHandlerFunc: middleware.StrictResponseError(d.Log),
	})
	guard := d.Guard.Middleware(policy.NewTable(Policies()...))
	gen.HandlerWithOptions(strict, gen.ChiServerOptions{
		BaseRouter:       r,
		Middlewares:      []gen.MiddlewareFunc{middleware.BodyLimit(config.JobBodyMaxBytes), guard},
		ErrorHandlerFunc: middleware.StrictRequestError,
	})
}

func ConnectRoutes(mux *http.ServeMux, d Deps) {
	options := []connect.HandlerOption{
		d.Gate.RequestGate(policy.NewTable(Procedures()...)),
		connect.WithReadMaxBytes(config.RunnerReadMaxBytes),
		connect.WithInterceptors(runnerauth.Interceptor(d.Log)),
	}
	mux.Handle(runnerv1connect.NewLeaseServiceHandler(connectHandler{service: d.Service}, options...))
}
