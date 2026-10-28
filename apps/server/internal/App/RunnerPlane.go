package app

import (
	"net/http"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	runners "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Runners"
)

func RunnerPlane(o Options) http.Handler {
	mux := http.NewServeMux()
	runners.ConnectRoutes(mux, runners.Deps{Service: o.Runners, Gate: o.Gate, Log: o.Log})
	h := http.StripPrefix(config.RunnerAPIPrefix, mux)
	stack := []func(http.Handler) http.Handler{
		middleware.RequestID,
		middleware.ClientIP(o.Config.TrustedProxies),
		middleware.AccessLog(o.Log),
		middleware.Recover(o.Log),
		middleware.NoStore,
		middleware.BodyLimit(config.RunnerBodyMaxBytes),
	}
	for i := len(stack) - 1; i >= 0; i-- {
		h = stack[i](h)
	}
	return h
}

func RunnerPlaneServer(o Options) http.Handler {
	root := http.NewServeMux()
	root.Handle(config.RunnerAPIPrefix+"/", RunnerPlane(o))
	return root
}
