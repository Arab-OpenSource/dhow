// Package web serves the browser UI.
//
// The UI (apps/web) is built in CI and embedded into the server binary. Until that lands, Handler answers
// every UI route with 404 so the server never serves anything it did not explicitly ship.
package web

import (
	"net/http"

	"github.com/Arab-OpenSource/dhow/apps/server/internal/httpx"
)

// Handler returns the handler for non-API routes.
//
// Integration point for the web build: replace the body with a handler that serves the embedded
// apps/web/dist (index.html for client-side routes, hashed assets with long cache headers).
func Handler() http.Handler {
	return http.HandlerFunc(httpx.NotFound)
}
