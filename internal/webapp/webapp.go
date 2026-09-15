// Package webapp serves the embedded production frontend from the application
// listener. It owns SPA route resolution, static asset delivery, cache policy,
// and frontend-specific security headers. Development builds use the Vite
// development server instead and NewEmbedded returns nil.
package webapp

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
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
	nonceMeta      = "upaste-csp-nonce"
)

// Options lets the application listener expose the /admin SPA route and adapt
// CSP only for the configured challenge provider.
type Options struct {
	AdminEnabled      bool
	ChallengeProvider string
	CapEndpoint       string
}

type Handler struct {
	assets       fs.FS
	index        []byte
	adminEnabled bool
	provider     string
	capOrigin    string
}

// New validates a frontend bundle and returns a handler for it.
func New(assets fs.FS) (*Handler, error) {
	return NewWithConfig(assets, Options{})
}

// NewWithOptions retains the Phase 8 helper shape for existing callers.
func NewWithOptions(assets fs.FS, adminEnabled bool) (*Handler, error) {
	return NewWithConfig(assets, Options{AdminEnabled: adminEnabled})
}

// NewWithConfig validates a frontend bundle and applies runtime provider CSP.
func NewWithConfig(assets fs.FS, options Options) (*Handler, error) {
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
	return &Handler{
		assets:       assets,
		index:        index,
		adminEnabled: options.AdminEnabled,
		provider:     options.ChallengeProvider,
		capOrigin:    strings.TrimSuffix(options.CapEndpoint, "/"),
	}, nil
}

func (handler *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	nonce := newNonce()
	handler.setSecurityHeaders(w, handler.contentSecurityPolicy(nonce))

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
		handler.serveIndex(w, r, nonce)
		return
	}
	if name, ok := handler.lookupAsset(r.URL.Path); ok {
		handler.serveAsset(w, r, name)
		return
	}
	http.NotFound(w, r)
}

func (handler *Handler) serveIndex(w http.ResponseWriter, r *http.Request, nonce string) {
	body := handler.index
	if handler.provider == "cap" && nonce != "" {
		meta := []byte(`<meta name="` + nonceMeta + `" content="` + nonce + `">`)
		body = bytes.Replace(body, []byte("</head>"), append(meta, []byte("</head>")...), 1)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", htmlCache)
	http.ServeContent(w, r, indexFile, time.Time{}, bytes.NewReader(body))
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
	case "/admin":
		return handler.adminEnabled
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

func (handler *Handler) setSecurityHeaders(w http.ResponseWriter, csp string) {
	header := w.Header()
	header.Set("X-Content-Type-Options", "nosniff")
	header.Set("Referrer-Policy", "no-referrer")
	header.Set("X-Frame-Options", "DENY")
	header.Set("Content-Security-Policy", csp)
}

func (handler *Handler) contentSecurityPolicy(nonce string) string {
	script := []string{"'self'"}
	style := []string{"'self'"}
	connect := []string{"'self'"}
	frame := []string{"'self'"}
	worker := []string{"'self'"}
	img := []string{"'self'", "data:"}

	switch handler.provider {
	case "turnstile":
		// Cloudflare's documented explicit-render origins only.
		script = append(script, "https://challenges.cloudflare.com")
		frame = append(frame, "https://challenges.cloudflare.com")
		connect = append(connect, "https://challenges.cloudflare.com")
	case "cap":
		// The pinned widget and its WASM are bundled same-origin. The external
		// Cap Standalone origin only carries challenge/redeem network traffic.
		if handler.capOrigin != "" {
			connect = append(connect, handler.capOrigin)
		}
		// The pinned Cap widget compiles fetched WASM and may create workers.
		script = append(script, "'wasm-unsafe-eval'")
		worker = append(worker, "blob:")
		if nonce != "" {
			script = append(script, "'nonce-"+nonce+"'")
			style = append(style, "'nonce-"+nonce+"'")
		}
	}
	return "default-src 'self'; base-uri 'none'; object-src 'none'; frame-ancestors 'none'; form-action 'self'; " +
		"script-src " + strings.Join(script, " ") + "; " +
		"style-src " + strings.Join(style, " ") + "; " +
		"img-src " + strings.Join(img, " ") + "; " +
		"font-src 'self'; " +
		"connect-src " + strings.Join(connect, " ") + "; " +
		"frame-src " + strings.Join(frame, " ") + "; " +
		"worker-src " + strings.Join(worker, " ")
}

func newNonce() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(value)
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
