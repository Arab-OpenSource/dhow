package runners

import (
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"github.com/go-chi/chi/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	runnerauth "github.com/Khaymah/dhow/apps/server/internal/Core/RunnerAuth"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Runners/Gen"
	"github.com/Khaymah/dhow/packages/proto/gen/go/dhow/runner/v1/runnerv1connect"
)

type Deps struct {
	Service *Service
	Guard   *auth.Guard
	Gate    *runnerauth.Gate
	Log     *slog.Logger
}

func Policies() []policy.Rule {
	return []policy.Rule{
		policy.Organization(http.MethodGet, "/organizations/{id}/runners", policy.RunnerRead, "id").AllowTokens(),
		policy.Organization(http.MethodGet, "/organizations/{id}/runners/{runnerId}", policy.RunnerRead, "id").AllowTokens(),
		policy.Organization(http.MethodPatch, "/organizations/{id}/runners/{runnerId}", policy.RunnerManage, "id").AllowTokens(),
		policy.Organization(http.MethodPost, "/organizations/{id}/runners/{runnerId}/revoke", policy.RunnerManage, "id").AllowTokens(),
		policy.Organization(http.MethodGet, "/organizations/{id}/runner-registration-tokens", policy.RunnerManage, "id").AllowTokens(),
		policy.Organization(http.MethodPost, "/organizations/{id}/runner-registration-tokens", policy.RunnerManage, "id").AllowTokens(),
		policy.Organization(http.MethodPost, "/organizations/{id}/runner-registration-tokens/{tokenId}/revoke", policy.RunnerManage, "id").AllowTokens(),
	}
}

func Procedures() []policy.Rule {
	return []policy.Rule{
		policy.Runner(runnerv1connect.RegistrationServiceRegisterProcedure, policy.RunnerRegistration),
		policy.Runner(runnerv1connect.AuthServiceChallengeProcedure, policy.RunnerChallenge),
		policy.Runner(runnerv1connect.AuthServiceAuthenticateProcedure, policy.RunnerSigned),
		policy.Runner(runnerv1connect.AuthServiceRotateKeysProcedure, policy.RunnerSession),
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

func ConnectRoutes(mux *http.ServeMux, d Deps) {
	options := []connect.HandlerOption{
		d.Gate.RequestGate(policy.NewTable(Procedures()...)),
		connect.WithReadMaxBytes(config.RunnerReadMaxBytes),
		connect.WithInterceptors(runnerauth.Interceptor(d.Log)),
	}
	handler := connectHandler{service: d.Service}
	mux.Handle(runnerv1connect.NewRegistrationServiceHandler(handler, options...))
	mux.Handle(runnerv1connect.NewAuthServiceHandler(handler, options...))
}
