package app

import (
	"bytes"
	"embed"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

//go:embed all:WebDist
var webDist embed.FS

const (
	webRoot         = "WebDist"
	indexFile       = "index.html"
	assetsDir       = "assets"
	cacheImmutable  = "public, max-age=31536000, immutable"
	cacheNoStore    = "no-store"
	cacheRevalidate = "no-cache"
)

type distFS struct {
	files fs.FS
	root  string
}

func (d distFS) Open(name string) (fs.File, error) {
	return d.files.Open(path.Join(d.root, name))
}

func WebHandler() http.Handler {
	return NewWebHandler(distFS{files: webDist, root: webRoot})
}

func NewWebHandler(files fs.FS) http.Handler {
	if _, err := fs.Stat(files, indexFile); err != nil {
		return http.HandlerFunc(notFound)
	}
	return webServer{files: files}
}

type webServer struct {
	files fs.FS
}

func (s webServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w, r)
		return
	}
	name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if name == "" {
		name = indexFile
	}
	if s.serve(w, r, name) {
		return
	}
	if isAssetPath(name) || path.Ext(name) != "" || !s.serve(w, r, indexFile) {
		notFound(w, r)
	}
}

func (s webServer) serve(w http.ResponseWriter, r *http.Request, name string) bool {
	data, err := fs.ReadFile(s.files, name)
	if err != nil {
		return false
	}
	h := w.Header()
	h.Set("Cache-Control", cacheControl(name))
	if ct := contentType(name); ct != "" {
		h.Set("Content-Type", ct)
	}
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	return true
}

func isAssetPath(name string) bool {
	return name == assetsDir || strings.HasPrefix(name, assetsDir+"/")
}

func cacheControl(name string) string {
	switch {
	case name == indexFile:
		return cacheNoStore
	case isAssetPath(name):
		return cacheImmutable
	default:
		return cacheRevalidate
	}
}

func contentType(name string) string {
	ext := path.Ext(name)
	switch ext {
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	default:
		return mime.TypeByExtension(ext)
	}
}
