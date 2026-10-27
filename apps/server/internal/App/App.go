package app

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	response "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Response"
	system "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"
)

type Options struct {
	Config config.Config
	Log    *slog.Logger
	System system.Deps
	Web    http.Handler
}

func NewRouter(o Options) http.Handler {
	r := chi.NewRouter()
	r.Use(
		middleware.RequestID,
		middleware.ClientIP(o.Config.TrustedProxies),
		middleware.Lang,
		middleware.AccessLog(o.Log),
		middleware.Recover(o.Log),
		middleware.SecurityHeaders(o.Config.PublicURL.Scheme == "https"),
		middleware.BodyLimit(config.MaxBodyBytes),
	)
	r.MethodNotAllowed(methodNotAllowed)

	r.Route("/api/v1", func(api chi.Router) {
		api.Use(middleware.Timeout(config.APITimeout), middleware.NoStore)
		api.NotFound(notFound)
		system.Routes(api, o.System)
	})

	web := o.Web
	if web == nil {
		web = WebHandler()
	}
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api" || strings.HasPrefix(req.URL.Path, "/api/") {
			notFound(w, req)
			return
		}
		web.ServeHTTP(w, req)
	})
	return r
}

func notFound(w http.ResponseWriter, r *http.Request) {
	response.Error(w, r, apperrors.NotFound())
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	response.Error(w, r, apperrors.MethodNotAllowed())
}
