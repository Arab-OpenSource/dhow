package app

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	config "github.com/Arab-OpenSource/dhow/apps/server/internal/Config"
	auth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Auth"
	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	middleware "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Middleware"
	policy "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Policy"
	response "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Response"
	runnerauth "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/RunnerAuth"
	audit "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Audit"
	identity "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Identity"
	jobs "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Jobs"
	runners "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Runners"
	session "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/Session"
	system "github.com/Arab-OpenSource/dhow/apps/server/internal/Modules/System"
)

type Options struct {
	Config   config.Config
	Log      *slog.Logger
	System   system.Deps
	Identity *identity.Service
	Session  *session.Service
	Audit    *audit.Service
	Runners  *runners.Service
	Jobs     *jobs.Service
	Gate     *runnerauth.Gate
	CSRF     *auth.CSRF
	Now      func() time.Time
	Web      http.Handler
}

func Policies() []policy.Rule {
	var rules []policy.Rule
	rules = append(rules, system.Policies()...)
	rules = append(rules, identity.Policies()...)
	rules = append(rules, session.Policies()...)
	rules = append(rules, audit.Policies()...)
	rules = append(rules, runners.Policies()...)
	rules = append(rules, jobs.Policies()...)
	return rules
}

func RunnerPolicies() []policy.Rule {
	return append(runners.Procedures(), jobs.Procedures()...)
}

type noSessions struct{}

func (noSessions) Resolve(context.Context, []byte, time.Time) (auth.Resolved, error) {
	return auth.Resolved{}, auth.ErrNoSession
}

func (o Options) guard() *auth.Guard {
	var resolver auth.Resolver = noSessions{}
	if o.Session != nil {
		resolver = o.Session
	}
	csrf := o.CSRF
	if csrf == nil {
		csrf = auth.NewCSRF(o.Config.SecretKey)
	}
	return auth.NewGuard(auth.GuardOptions{Resolver: resolver, CSRF: csrf, PublicURL: o.Config.PublicURL, Log: o.Log, Now: o.Now})
}

func NewRouter(o Options) http.Handler {
	public := publicRouter(o)
	if o.Runners == nil || o.Config.RunnerPlaneSeparate() {
		return public
	}
	root := chi.NewRouter()
	root.Mount(config.RunnerAPIPrefix, RunnerPlane(o))
	root.Mount("/", public)
	return root
}

func publicRouter(o Options) chi.Router {
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

	guard := o.guard()
	r.Route(config.APIPrefix, func(api chi.Router) {
		api.Use(middleware.Timeout(config.APITimeout), middleware.NoStore)
		api.NotFound(notFound)
		o.System.Guard = guard
		system.Routes(api, o.System)
		if o.Identity != nil {
			identity.Routes(api, identity.Deps{Service: o.Identity, Guard: guard, Log: o.Log})
		}
		if o.Session != nil {
			session.Routes(api, session.Deps{Service: o.Session, Guard: guard, Log: o.Log})
		}
		if o.Audit != nil {
			audit.Routes(api, audit.Deps{Service: o.Audit, Guard: guard, Log: o.Log})
		}
		if o.Runners != nil {
			runners.Routes(api, runners.Deps{Service: o.Runners, Guard: guard, Log: o.Log})
		}
		if o.Jobs != nil {
			jobs.Routes(api, jobs.Deps{Service: o.Jobs, Guard: guard, Log: o.Log})
		}
	})

	web := o.Web
	if web == nil {
		web = WebHandler()
	}
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if isAPIPath(req.URL.Path) {
			notFound(w, req)
			return
		}
		web.ServeHTTP(w, req)
	})
	return r
}

func isAPIPath(p string) bool {
	for _, prefix := range []string{"/api", config.RunnerAPIPrefix} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return true
		}
	}
	return false
}

func notFound(w http.ResponseWriter, r *http.Request) {
	response.Error(w, r, apperrors.NotFound())
}

func methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	response.Error(w, r, apperrors.MethodNotAllowed())
}
