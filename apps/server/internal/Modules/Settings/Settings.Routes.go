package settings

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	config "github.com/Khaymah/dhow/apps/server/internal/Config"
	auth "github.com/Khaymah/dhow/apps/server/internal/Core/Auth"
	middleware "github.com/Khaymah/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Khaymah/dhow/apps/server/internal/Core/Policy"
	gen "github.com/Khaymah/dhow/apps/server/internal/Modules/Settings/Gen"
)

type Deps struct {
	Service *Service
	Guard   *auth.Guard
	Log     *slog.Logger
}

const settingsPath = "/instance/settings"

func Policies() []policy.Rule {
	return []policy.Rule{
		policy.Instance(http.MethodGet, settingsPath, policy.InstanceSettingsManage),
		policy.Instance(http.MethodPut, settingsPath+"/mail", policy.InstanceSettingsManage),
		policy.Instance(http.MethodPost, settingsPath+"/mail/test", policy.InstanceSettingsManage),
		policy.Instance(http.MethodPut, settingsPath+"/organizations", policy.InstanceSettingsManage),
		policy.Instance(http.MethodPut, settingsPath+"/passwords", policy.InstanceSettingsManage),
		policy.Instance(http.MethodPut, settingsPath+"/rate-limits", policy.InstanceSettingsManage),
		policy.Instance(http.MethodPut, settingsPath+"/logs", policy.InstanceSettingsManage),
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
