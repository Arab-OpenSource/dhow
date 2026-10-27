package middleware

import (
	"log/slog"
	"net/http"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	response "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Response"
)

func StrictRequestError(w http.ResponseWriter, r *http.Request, _ error) {
	response.Error(w, r, apperrors.BadRequest())
}

func StrictResponseError(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		ae := apperrors.From(err)
		if ae.Status >= http.StatusInternalServerError {
			log.ErrorContext(r.Context(), "api handler failed", "request_id", RequestIDFrom(r.Context()), "error", err)
		}
		response.Error(w, r, ae)
	}
}
