package fileapi

import (
	"encoding/hex"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DejavuMoe/uPaste/internal/abuse"
	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/share"
)

type API struct {
	shares *share.Service
	log    *slog.Logger
	abuse  *abuse.Control
}

func New(shares *share.Service, log *slog.Logger) http.Handler {
	return NewWithAbuse(shares, log, abuse.New(abuse.Config{Disabled: true}))
}
func NewWithAbuse(shares *share.Service, log *slog.Logger, control *abuse.Control) http.Handler {
	api := &API{shares: shares, log: log, abuse: control}
	return securityHeaders(http.HandlerFunc(api.route))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/f" && !strings.HasPrefix(r.URL.Path, "/f/") {
			next.ServeHTTP(w, r)
			return
		}
		secure := fileResponseWriter{ResponseWriter: w}
		secure.apply()
		next.ServeHTTP(&secure, r)
	})
}

type fileResponseWriter struct{ http.ResponseWriter }
type headResponseWriter struct{ http.ResponseWriter }

func (writer headResponseWriter) Write(value []byte) (int, error) { return len(value), nil }
func (writer headResponseWriter) Unwrap() http.ResponseWriter     { return writer.ResponseWriter }
func (writer *fileResponseWriter) Unwrap() http.ResponseWriter    { return writer.ResponseWriter }

func (writer *fileResponseWriter) WriteHeader(status int) {
	writer.apply()
	writer.ResponseWriter.WriteHeader(status)
}

func (writer *fileResponseWriter) Write(value []byte) (int, error) {
	writer.apply()
	return writer.ResponseWriter.Write(value)
}

func (writer *fileResponseWriter) apply() {
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Referrer-Policy", "no-referrer")
	writer.Header().Set("Content-Security-Policy", "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; sandbox")
	writer.Header().Set("X-Frame-Options", "DENY")
}

func (api *API) route(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodHead {
		w = headResponseWriter{ResponseWriter: w}
	}
	if r.URL.Path != "/f" && !strings.HasPrefix(r.URL.Path, "/f/") {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !api.abuse.Allow(r, abuse.Read) {
		api.rateLimited(w, 60)
		return
	}
	if !api.abuse.TryDownload() {
		api.rateLimited(w, 1)
		return
	}
	defer api.abuse.ReleaseDownload()
	idValue := strings.TrimPrefix(r.URL.Path, "/f/")
	if idValue == "" || strings.Contains(idValue, "/") {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	id, err := capability.ParseShareID(idValue)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}
	value, object, err := api.shares.OpenFile(r.Context(), id)
	if err != nil {
		api.error(w, id.String(), err)
		return
	}
	defer object.Close()
	if err := http.NewResponseController(w).SetWriteDeadline(time.Now().Add(share.FileTransferDeadline)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		api.log.Error("request failed", "operation", "extend File download deadline", "share_id", id.String(), "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", value.File.MediaType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": value.File.Filename}))
	w.Header().Set("ETag", `"`+hex.EncodeToString(value.File.SHA256[:])+`"`)
	w.Header().Set("Content-Length", strconv.FormatInt(value.File.Size, 10))
	http.ServeContent(w, r, value.File.Filename, value.CreatedAt, object)
}

func (api *API) rateLimited(w http.ResponseWriter, retry int) {
	w.Header().Set("Retry-After", strconv.Itoa(retry))
	http.Error(w, "too many requests", http.StatusTooManyRequests)
}

func (api *API) error(w http.ResponseWriter, id string, err error) {
	switch {
	case errors.Is(err, share.ErrNotFound):
		http.Error(w, "file not found", http.StatusNotFound)
	case errors.Is(err, share.ErrExpired):
		http.Error(w, "file expired", http.StatusGone)
	default:
		api.log.Error("request failed", "operation", "serve File", "share_id", id, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
