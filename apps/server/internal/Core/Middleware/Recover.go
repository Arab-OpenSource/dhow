package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	response "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Response"
)

func Recover(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					handlePanic(log, w, r, rec)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

func handlePanic(log *slog.Logger, w http.ResponseWriter, r *http.Request, rec any) {
	ctx := r.Context()
	if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
		log.WarnContext(ctx, "handler aborted", "request_id", RequestIDFrom(ctx), "path", r.URL.Path)
		return
	}
	log.ErrorContext(ctx, "panic serving request",
		"request_id", RequestIDFrom(ctx), "path", r.URL.Path, "panic", rec, "stack", string(debug.Stack()))
	response.Error(w, r, apperrors.Internal(fmt.Errorf("panic: %v", rec)))
}
