package web

import (
	"net/http"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/httpx"
)

func Handler() http.Handler {
	return http.HandlerFunc(httpx.NotFound)
}
