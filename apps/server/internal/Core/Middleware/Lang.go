package middleware

import (
	"net/http"

	i18n "github.com/Khaymah/dhow/apps/server/internal/Core/I18n"
)

func Lang(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locale := i18n.Parse(r.Header.Get("Accept-Language"))
		w.Header().Set("Content-Language", string(locale))
		next.ServeHTTP(w, r.WithContext(i18n.WithLocale(r.Context(), locale)))
	})
}
