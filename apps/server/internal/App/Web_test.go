package app_test

import (
	"encoding/json"
	"net/http"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	app "github.com/Khaymah/dhow/apps/server/internal/App"
	system "github.com/Khaymah/dhow/apps/server/internal/Modules/System"
	support "github.com/Khaymah/dhow/apps/server/internal/Modules/System/Test/Support"
)

const (
	indexBody       = "<!doctype html><title>Dhow</title><div id=\"root\"></div>"
	immutableCache  = "public, max-age=31536000, immutable"
	noStoreCache    = "no-store"
	revalidateCache = "no-cache"
)

func distFiles() fstest.MapFS {
	return fstest.MapFS{
		"index.html":                 {Data: []byte(indexBody)},
		"ThemeInit.js":               {Data: []byte("document.documentElement.dir = 'rtl';")},
		"Favicon.svg":                {Data: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>")},
		"assets/index-abc123.js":     {Data: []byte("export {};")},
		"assets/index-abc123.css":    {Data: []byte("body{margin:0}")},
		"assets/poppins-xyz789.woff": {Data: []byte("wOFF")},
		"assets/plex-xyz789.woff2":   {Data: []byte("wOF2")},
	}
}

func webRouter(files fstest.MapFS) http.Handler {
	return support.RouterWithWeb(system.Deps{}, app.NewWebHandler(files))
}

func TestWebAssetCaching(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantCache   string
		wantType    string
		wantContain string
	}{
		{"root is the app and never cached", "/", noStoreCache, "text/html", indexBody},
		{"index file is never cached", "/index.html", noStoreCache, "text/html", indexBody},
		{"hashed script is immutable", "/assets/index-abc123.js", immutableCache, "text/javascript", "export"},
		{"hashed stylesheet is immutable", "/assets/index-abc123.css", immutableCache, "text/css", "margin"},
		{"hashed woff2 font is immutable", "/assets/plex-xyz789.woff2", immutableCache, "font/woff2", "wOF2"},
		{"hashed woff font is immutable", "/assets/poppins-xyz789.woff", immutableCache, "font/woff", "wOFF"},
		{"language init script must revalidate", "/ThemeInit.js", revalidateCache, "text/javascript", "rtl"},
		{"favicon must revalidate", "/Favicon.svg", revalidateCache, "image/svg+xml", "<svg"},
	}
	h := webRouter(distFiles())
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := support.Do(h, http.MethodGet, tt.path, nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.wantCache, rec.Header().Get("Cache-Control"))
			assert.Contains(t, rec.Header().Get("Content-Type"), tt.wantType)
			assert.Contains(t, rec.Body.String(), tt.wantContain)
		})
	}
}

func TestWebSPAFallback(t *testing.T) {
	paths := []string{"/dashboard", "/projects/42/settings", "/some/deep/path/", "/?panel=system-status", "/assets/../../etc/passwd"}
	h := webRouter(distFiles())
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			rec := support.Do(h, http.MethodGet, p, nil)
			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, indexBody, rec.Body.String())
			assert.Equal(t, noStoreCache, rec.Header().Get("Cache-Control"))
			assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
		})
	}
}

func TestWebMissingFilesAreNotTheApp(t *testing.T) {
	paths := []string{"/assets/missing-abc.js", "/assets/", "/assets", "/robots.txt", "/static/logo.png"}
	h := webRouter(distFiles())
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			rec := support.Do(h, http.MethodGet, p, nil)
			require.Equal(t, http.StatusNotFound, rec.Code)
			assertEnvelopeNotFound(t, rec.Header().Get("Content-Type"), rec.Body.Bytes())
		})
	}
}

func TestAPIPathsStayJSONEvenWithTheAppEmbedded(t *testing.T) {
	paths := []string{"/api", "/api/", "/api/v1/nope", "/api/v1/", "/api/v2/meta", "/api/anything/else"}
	h := webRouter(distFiles())
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			rec := support.Do(h, http.MethodGet, p, nil)
			require.Equal(t, http.StatusNotFound, rec.Code)
			assertEnvelopeNotFound(t, rec.Header().Get("Content-Type"), rec.Body.Bytes())
			assert.NotContains(t, rec.Body.String(), "<!doctype")
		})
	}
}

func TestAPIStillAnswersWithTheAppEmbedded(t *testing.T) {
	rec := support.Do(webRouter(distFiles()), http.MethodGet, "/api/v1/meta", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, noStoreCache, rec.Header().Get("Cache-Control"))
	assert.Contains(t, rec.Header().Get("Content-Type"), "application/json")
}

func TestWebOnlyServesReadMethods(t *testing.T) {
	h := webRouter(distFiles())
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			rec := support.Do(h, method, "/dashboard", nil)
			require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
			assert.Contains(t, rec.Body.String(), "METHOD_NOT_ALLOWED")
		})
	}
	head := support.Do(h, http.MethodHead, "/", nil)
	require.Equal(t, http.StatusOK, head.Code)
	assert.Empty(t, head.Body.String())
	assert.Equal(t, noStoreCache, head.Header().Get("Cache-Control"))
}

func TestWebKeepsTheStrictSecurityHeaders(t *testing.T) {
	paths := []string{"/", "/dashboard", "/assets/index-abc123.js", "/ThemeInit.js"}
	h := webRouter(distFiles())
	for _, p := range paths {
		t.Run(p, func(t *testing.T) {
			rec := support.Do(h, http.MethodGet, p, nil)
			csp := rec.Header().Get("Content-Security-Policy")
			assert.Contains(t, csp, "script-src 'self'")
			assert.Contains(t, csp, "style-src 'self'")
			assert.NotContains(t, csp, "unsafe-inline")
			assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
		})
	}
}

func TestWithoutABuiltAppEveryNonAPIPathIsAJSONNotFound(t *testing.T) {
	h := webRouter(fstest.MapFS{})
	for _, p := range []string{"/", "/dashboard", "/assets/index-abc123.js"} {
		t.Run(p, func(t *testing.T) {
			rec := support.Do(h, http.MethodGet, p, nil)
			require.Equal(t, http.StatusNotFound, rec.Code)
			assertEnvelopeNotFound(t, rec.Header().Get("Content-Type"), rec.Body.Bytes())
		})
	}
}

func TestEmbeddedHandlerKeepsAPIPathsJSON(t *testing.T) {
	h := support.RouterWithWeb(system.Deps{}, app.WebHandler())
	rec := support.Do(h, http.MethodGet, "/api/v1/nope", nil)
	require.Equal(t, http.StatusNotFound, rec.Code)
	assertEnvelopeNotFound(t, rec.Header().Get("Content-Type"), rec.Body.Bytes())
}

func assertEnvelopeNotFound(t *testing.T, contentType string, body []byte) {
	t.Helper()
	assert.Contains(t, contentType, "application/json")
	var env struct {
		Success bool
		Error   *struct{ Code string }
	}
	require.NoError(t, json.Unmarshal(body, &env), "body is not the JSON envelope: %q", body)
	assert.False(t, env.Success)
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}
