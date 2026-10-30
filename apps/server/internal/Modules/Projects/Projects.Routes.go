package projects

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Projects/Gen"
)

const (
	projectsPath = "/organizations/{id}/projects"
	projectPath  = projectsPath + "/{projectId}"
)

type Deps struct {
	Service *Service
	Guard   *auth.Guard
	Log     *slog.Logger
}

func Policies() []policy.Rule {
	return []policy.Rule{
		policy.Organization(http.MethodGet, projectsPath, policy.ProjectRead, "id").AllowTokens(),
		policy.Organization(http.MethodPost, projectsPath, policy.ProjectManage, "id").AllowTokens(),
		policy.Project(http.MethodGet, projectPath, policy.ProjectRead, "id", "projectId").AllowTokens(),
		policy.Project(http.MethodPatch, projectPath, policy.ProjectManage, "id", "projectId").AllowTokens(),
		policy.Project(http.MethodPost, projectPath+"/archive", policy.ProjectManage, "id", "projectId").AllowTokens(),
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
