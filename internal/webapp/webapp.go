// Package webapp serves the embedded production frontend from the application
// listener. It owns SPA route resolution, static asset delivery, cache policy,
// and frontend-specific security headers. Development builds use the Vite
// development server instead and NewEmbedded returns nil.
package webapp

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	indexFile = "index.html"

	immutableCache = "public, max-age=31536000, immutable"
	htmlCache      = "no-store"

	contentSecurityPolicy = "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; font-src 'self'; connect-src 'self'"
)

// Handler serves an immutable frontend bundle rooted at index.html, plus the
// two frozen SPA route shapes /s/:id and /manage/:id.
type Handler struct {
	assets fs.FS
	index  []byte
}

// New validates a frontend bundle and returns a handler for it.
func New(assets fs.FS) (*Handler, error) {
	if assets == nil {
		return nil, errors.New("frontend assets are unavailable")
	}
	index, err := fs.ReadFile(assets, indexFile)
	if err != nil {
		return nil, fmt.Errorf("read frontend index: %w", err)
	}
	if len(index) == 0 {
		return nil, errors.New("frontend index is empty")
	}
	return &Handler{assets: assets, index: index}, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if handler.isAppRoute(r.URL.Path) || handler.assetExists(r.URL.Path) {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		http.NotFound(w, r)
		return
	}

	if handler.isAppRoute(r.URL.Path) {
		handler.serveIndex(w, r)
		return
	}
	if name, ok := handler.lookupAsset(r.URL.Path); ok {
		handler.serveAsset(w, r, name)
		return
	}
	http.NotFound(w, r)
}

func (handler *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", htmlCache)
	http.ServeContent(w, r, indexFile, time.Time{}, bytes.NewReader(handler.index))
}

func (handler *Handler) serveAsset(w http.ResponseWriter, r *http.Request, name string) {
	data, err := fs.ReadFile(handler.assets, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasPrefix(name, "assets/") {
		w.Header().Set("Cache-Control", immutableCache)
	} else {
		w.Header().Set("Cache-Control", htmlCache)
	}
	w.Header().Set("Content-Type", contentTypeFor(name))
	http.ServeContent(w, r, path.Base(name), time.Time{}, bytes.NewReader(data))
}

// isAppRoute reports whether the request path is one of the frozen SPA entry
// shapes. Anything else must not receive index.html.
func (handler *Handler) isAppRoute(requestPath string) bool {
	switch requestPath {
	case "/", "/" + indexFile:
		return true
	}
	for _, prefix := range []string{"/s/", "/manage/"} {
		if strings.HasPrefix(requestPath, prefix) {
			remainder := strings.TrimPrefix(requestPath, prefix)
			return remainder != "" && !strings.Contains(remainder, "/")
		}
	}
	return false
}

func (handler *Handler) assetExists(requestPath string) bool {
	_, ok := handler.lookupAsset(requestPath)
	return ok
}

// lookupAsset resolves an embedded regular file. The embedded asset set is
// generated from the Vite build, never supplied by request input.
func (handler *Handler) lookupAsset(requestPath string) (string, bool) {
	name := strings.TrimPrefix(requestPath, "/")
	if name == "" || !fs.ValidPath(name) {
		return "", false
	}
	info, err := fs.Stat(handler.assets, name)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	return name, true
}

func setSecurityHeaders(w http.ResponseWriter) {
	header := w.Header()
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Content-Security-Policy", contentSecurityPolicy)
}

func contentTypeFor(name string) string {
	switch path.Ext(name) {
	case ".html":
		return "text/html; charset=utf-8"
	case ".js", ".mjs":
		return "text/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".json", ".map":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".wasm":
		return "application/wasm"
	}
	if detected := mime.TypeByExtension(path.Ext(name)); detected != "" {
		return detected
	}
	return "application/octet-stream"
}
