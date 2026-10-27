package auth

import (
	"net/http"
	"time"
)

func SessionCookie(name, value string, maxAge time.Duration) string {
	c := http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	return c.String()
}

func ClearedSessionCookie(name string) string {
	c := http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	return c.String()
}
