package middleware

import (
	"net/http"

	apperrors "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Errors"
	response "github.com/Arab-OpenSource/dhow/apps/server/internal/Core/Response"
)

func BodyLimit(n int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > n {
				response.Error(w, r, apperrors.PayloadTooLarge())
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}
