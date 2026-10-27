// Package api implements the public REST API defined in packages/openapi/openapi.yaml.
package api

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/api/gen"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/db/sqlc"
	"github.com/Arab-OpenSource/dhow/apps/server/internal/httpx"
	"github.com/Arab-OpenSource/dhow/internal/buildinfo"
)

// Pinger checks database connectivity.
type Pinger interface {
	Ping(ctx context.Context) error
}

// MigrationStatus reports whether the schema is fully migrated.
type MigrationStatus interface {
	Current(ctx context.Context) (bool, error)
}

// Deps are the API's dependencies.
type Deps struct {
	DB         Pinger
	Migrations MigrationStatus
	Queries    sqlc.Querier
	Build      buildinfo.Info
	Log        *slog.Logger
}

// readinessTimeout bounds each readiness check so a stuck database fails the probe quickly.
const readinessTimeout = 2 * time.Second

// Server implements gen.StrictServerInterface.
type Server struct{ d Deps }

var _ gen.StrictServerInterface = (*Server)(nil)

// New returns the API implementation.
func New(d Deps) *Server { return &Server{d: d} }

// Handler mounts the API on a chi-compatible router at baseURL with generic error handling.
func Handler(s *Server, router chi.Router, baseURL string) http.Handler {
	strict := gen.NewStrictHandlerWithOptions(s, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
			httpx.WriteError(w, http.StatusBadRequest, "bad_request", "The request is not valid.")
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			s.d.Log.ErrorContext(r.Context(), "api handler failed", "request_id", httpx.RequestIDFrom(r.Context()), "error", err)
			httpx.WriteError(w, http.StatusInternalServerError, "internal", "Something went wrong. Try again later.")
		},
	})
	return gen.HandlerWithOptions(strict, gen.ChiServerOptions{
		BaseURL:    baseURL,
		BaseRouter: router,
		ErrorHandlerFunc: func(w http.ResponseWriter, _ *http.Request, _ error) {
			httpx.WriteError(w, http.StatusBadRequest, "bad_request", "The request is not valid.")
		},
	})
}

// GetLiveness answers while the process runs.
func (s *Server) GetLiveness(context.Context, gen.GetLivenessRequestObject) (gen.GetLivenessResponseObject, error) {
	return gen.GetLiveness200JSONResponse{Status: gen.Ok, Checks: []gen.HealthCheck{}}, nil
}

// GetReadiness checks the database and the schema version.
func (s *Server) GetReadiness(ctx context.Context, _ gen.GetReadinessRequestObject) (gen.GetReadinessResponseObject, error) {
	dbOK := s.check(ctx, "database", func(ctx context.Context) error { return s.d.DB.Ping(ctx) })
	migOK := dbOK && s.check(ctx, "migrations", func(ctx context.Context) error {
		ok, err := s.d.Migrations.Current(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return errNotMigrated
		}
		return nil
	})
	body := gen.HealthStatus{
		Status: gen.Ok,
		Checks: []gen.HealthCheck{{Name: gen.Database, Ok: dbOK}, {Name: gen.Migrations, Ok: migOK}},
	}
	if !dbOK || !migOK {
		body.Status = gen.Unavailable
		return gen.GetReadiness503JSONResponse(body), nil
	}
	return gen.GetReadiness200JSONResponse(body), nil
}

type readinessError string

func (e readinessError) Error() string { return string(e) }

const errNotMigrated = readinessError("schema is not at the latest migration")

func (s *Server) check(ctx context.Context, name string, fn func(context.Context) error) bool {
	ctx, cancel := context.WithTimeout(ctx, readinessTimeout)
	defer cancel()
	if err := fn(ctx); err != nil {
		s.d.Log.WarnContext(ctx, "readiness check failed", "check", name, "error", err)
		return false
	}
	return true
}

// GetMeta returns public instance metadata.
func (s *Server) GetMeta(ctx context.Context, _ gen.GetMetaRequestObject) (gen.GetMetaResponseObject, error) {
	inst, err := s.d.Queries.GetInstance(ctx)
	if err != nil {
		s.d.Log.ErrorContext(ctx, "load instance", "error", err)
		return gen.GetMetadefaultJSONResponse{
			StatusCode: http.StatusInternalServerError,
			Body:       gen.Error{Code: "internal", Message: "Something went wrong. Try again later."},
		}, nil
	}
	return gen.GetMeta200JSONResponse{
		Version:       s.d.Build.Version,
		Commit:        s.d.Build.Commit,
		SetupRequired: !inst.SetupCompletedAt.Valid,
	}, nil
}
