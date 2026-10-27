package server

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/api"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/config"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/httpx"
)

const (
	MaxBodyBytes      = 1 << 20
	APITimeout        = 15 * time.Second
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 60 * time.Second
	idleTimeout       = 120 * time.Second
	maxHeaderBytes    = 64 << 10
)

type Options struct {
	Config config.Config
	API    *api.Server
	Web    http.Handler
	Log    *slog.Logger
}

func Router(o Options) http.Handler {
	r := chi.NewRouter()
	r.Use(
		httpx.RequestID,
		httpx.ClientIP(o.Config.TrustedProxies),
		httpx.AccessLog(o.Log),
		httpx.Recover(o.Log),
		httpx.SecurityHeaders(o.Config.PublicURL.Scheme == "https"),
		httpx.BodyLimit(MaxBodyBytes),
	)
	r.MethodNotAllowed(httpx.MethodNotAllowed)

	r.Route("/api/v1", func(ar chi.Router) {
		ar.Use(httpx.Timeout(APITimeout), noStore)
		ar.NotFound(httpx.NotFound)
		api.Handler(o.API, ar, "")
	})

	web := o.Web
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api/") {
			httpx.NotFound(w, req)
			return
		}
		web.ServeHTTP(w, req)
	})
	return r
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func New(o Options) *http.Server {
	return &http.Server{
		Addr:              o.Config.ListenAddr,
		Handler:           Router(o),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
		ErrorLog:          slog.NewLogLogger(o.Log.Handler(), slog.LevelWarn),
	}
}
