// Package adminapi exposes the bounded Superadmin governance API. It can only
// inspect server-visible data and remove content; it cannot edit content,
// recover OwnerTokens, or decrypt encrypted Shares.
package adminapi

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/DejavuMoe/uPaste/internal/abuse"
	"github.com/DejavuMoe/uPaste/internal/admin"
	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/domain"
	"github.com/DejavuMoe/uPaste/internal/share"
)

const (
	basePath         = "/api/v1/admin"
	maxAdminBody     = 64 << 10
	defaultPageSize  = 50
	maxPageSize      = 100
	maxBulkDeleteIDs = 100
)

// ExpiredCleaner reuses the established maintenance purge semantics.
type ExpiredCleaner interface {
	CleanupExpired(ctx context.Context, now time.Time) (int, error)
}

type API struct {
	shares     *share.Service
	manager    *admin.Manager
	cleaner    ExpiredCleaner
	fileOrigin string
	trusted    []netip.Prefix
	log        *slog.Logger
	now        func() time.Time
}

func New(shares *share.Service, manager *admin.Manager, cleaner ExpiredCleaner, fileOrigin string, trusted []netip.Prefix, log *slog.Logger, now func() time.Time) http.Handler {
	if now == nil {
		now = time.Now
	}
	api := &API{shares: shares, manager: manager, cleaner: cleaner, fileOrigin: fileOrigin, trusted: trusted, log: log, now: now}
	return securityHeaders(http.HandlerFunc(api.route))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (api *API) route(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case basePath + "/session":
		api.session(w, r)
	case basePath + "/summary":
		api.summary(w, r)
	case basePath + "/shares":
		api.listShares(w, r)
	case basePath + "/shares/bulk-delete":
		api.bulkDelete(w, r)
	case basePath + "/cleanup/expired":
		api.cleanupExpired(w, r)
	default:
		if strings.HasPrefix(path, basePath+"/shares/") && !strings.Contains(strings.TrimPrefix(path, basePath+"/shares/"), "/") {
			api.shareItem(w, r, strings.TrimPrefix(path, basePath+"/shares/"))
			return
		}
		api.writeError(w, http.StatusNotFound, "not_found", "not found")
	}
}

func (api *API) session(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		api.login(w, r)
	case http.MethodGet:
		session, _, ok := api.authenticate(w, r, false)
		if !ok {
			return
		}
		api.writeJSON(w, http.StatusOK, map[string]any{
			"authenticated": true,
			"csrf":          session.CSRFToken,
			"expires_at":    session.ExpiresAt.UTC().Format(time.RFC3339Nano),
		})
	case http.MethodDelete:
		_, raw, ok := api.authenticate(w, r, true)
		if !ok {
			return
		}
		api.manager.Logout(raw)
		api.clearCookie(w)
		api.log.Info("admin logout")
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE")
		api.writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
	}
}

func (api *API) login(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Token string `json:"token"`
	}
	if err := decodeStrict(r, &request); err != nil || strings.TrimSpace(request.Token) == "" {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "request body is invalid")
		return
	}
	raw, csrf, err := api.manager.Login(request.Token, api.clientKey(r))
	switch {
	case errors.Is(err, admin.ErrRateLimited):
		w.Header().Set("Retry-After", "60")
		api.writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests")
		return
	case errors.Is(err, admin.ErrUnauthorized), err != nil:
		api.log.Warn("admin login", "result", "failure")
		api.writeError(w, http.StatusUnauthorized, "unauthorized", "authentication failed")
		return
	}
	expiresAt := api.now().Add(api.manager.SessionTTL())
	http.SetCookie(w, &http.Cookie{
		Name:     admin.SessionCookieName,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		Secure:   api.manager.SecureCookie() || r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(api.manager.SessionTTL().Seconds()),
		Expires:  expiresAt,
	})
	api.log.Info("admin login", "result", "success")
	api.writeJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"csrf":          csrf,
		"expires_at":    expiresAt.UTC().Format(time.RFC3339Nano),
	})
}

func (api *API) summary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, _, ok := api.authenticate(w, r, false); !ok {
		return
	}
	summary, err := api.shares.AdminSummary(r.Context())
	if err != nil {
		api.internalError(w, "admin summary", err)
		return
	}
	api.writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}

func (api *API) listShares(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.methodNotAllowed(w, http.MethodGet)
		return
	}
	if _, _, ok := api.authenticate(w, r, false); !ok {
		return
	}
	filter, err := parseFilter(r)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "invalid listing parameters")
		return
	}
	values, next, err := api.shares.AdminList(r.Context(), filter)
	if err != nil {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "invalid cursor or listing parameters")
		return
	}
	items := make([]adminListItemResponse, 0, len(values))
	for _, value := range values {
		items = append(items, api.listItemFromShare(value))
	}
	api.writeJSON(w, http.StatusOK, map[string]any{"shares": items, "next_cursor": next})
}

func (api *API) shareItem(w http.ResponseWriter, r *http.Request, idValue string) {
	id, err := capability.ParseShareID(idValue)
	if err != nil {
		api.writeError(w, http.StatusNotFound, "not_found", "share not found")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if _, _, ok := api.authenticate(w, r, false); !ok {
			return
		}
		value, err := api.shares.AdminGet(r.Context(), id)
		if err != nil {
			if errors.Is(err, share.ErrNotFound) {
				api.writeError(w, http.StatusNotFound, "not_found", "share not found")
				return
			}
			api.internalError(w, "admin read", err)
			return
		}
		api.writeJSON(w, http.StatusOK, map[string]any{"share": api.responseFromShare(value)})
	case http.MethodDelete:
		if _, _, ok := api.authenticate(w, r, true); !ok {
			return
		}
		if err := api.shares.AdminDelete(r.Context(), id); err != nil {
			var cleanup *share.CleanupError
			if errors.As(err, &cleanup) {
				api.log.Error("admin delete cleanup failed", "share_id", id.String(), "error", cleanup.Err)
			} else if errors.Is(err, share.ErrNotFound) {
				api.writeError(w, http.StatusNotFound, "not_found", "share not found")
				return
			} else {
				api.internalError(w, "admin delete", err)
				return
			}
		}
		api.log.Info("admin delete share", "share_id", id.String())
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, DELETE")
		api.writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
	}
}

func (api *API) bulkDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.methodNotAllowed(w, http.MethodPost)
		return
	}
	if _, _, ok := api.authenticate(w, r, true); !ok {
		return
	}
	var request struct {
		IDs []string `json:"ids"`
	}
	if err := decodeStrict(r, &request); err != nil || len(request.IDs) == 0 || len(request.IDs) > maxBulkDeleteIDs {
		api.writeError(w, http.StatusBadRequest, "invalid_request", "ids must contain 1-100 Share IDs")
		return
	}
	ids := make([]capability.ShareID, 0, len(request.IDs))
	failed := make([]string, 0)
	for _, value := range request.IDs {
		id, err := capability.ParseShareID(value)
		if err != nil {
			failed = append(failed, value)
			continue
		}
		ids = append(ids, id)
	}
	deleted := 0
	for _, id := range ids {
		if err := api.shares.AdminDelete(r.Context(), id); err != nil {
			failed = append(failed, id.String())
			continue
		}
		deleted++
	}
	api.log.Info("admin bulk delete", "requested", len(request.IDs), "deleted", deleted, "failed", len(failed))
	api.writeJSON(w, http.StatusOK, map[string]any{"deleted": deleted, "failed": failed})
}

func (api *API) cleanupExpired(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.methodNotAllowed(w, http.MethodPost)
		return
	}
	if _, _, ok := api.authenticate(w, r, true); !ok {
		return
	}
	if api.cleaner == nil {
		api.writeError(w, http.StatusNotImplemented, "unsupported", "cleanup is unavailable")
		return
	}
	purged, err := api.cleaner.CleanupExpired(r.Context(), api.now())
	if err != nil {
		api.internalError(w, "admin cleanup", err)
		return
	}
	api.log.Info("admin expired cleanup", "purged", purged)
	api.writeJSON(w, http.StatusOK, map[string]any{"purged": purged})
}

// authenticate validates the HttpOnly session cookie and, for state-changing
// requests, the session-bound CSRF header. It never returns the raw cookie.
func (api *API) authenticate(w http.ResponseWriter, r *http.Request, requireCSRF bool) (admin.Session, string, bool) {
	cookie, err := r.Cookie(admin.SessionCookieName)
	if err != nil || cookie.Value == "" {
		api.unauthorized(w)
		return admin.Session{}, "", false
	}
	session, ok := api.manager.Lookup(cookie.Value)
	if !ok {
		api.unauthorized(w)
		return admin.Session{}, "", false
	}
	if requireCSRF {
		values := r.Header.Values(admin.CSRFHeaderName)
		if len(values) != 1 || strings.TrimSpace(values[0]) == "" || !api.manager.CSRFMatches(cookie.Value, values[0]) {
			api.writeError(w, http.StatusForbidden, "csrf_failed", "CSRF validation failed")
			return admin.Session{}, "", false
		}
	}
	return session, cookie.Value, true
}

func (api *API) unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Cookie")
	api.writeError(w, http.StatusUnauthorized, "unauthorized", "admin authentication required")
}

func (api *API) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     admin.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   api.manager.SecureCookie(),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0).UTC(),
	})
}

func (api *API) methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	api.writeError(w, http.StatusMethodNotAllowed, "invalid_request", "method not allowed")
}

func (api *API) clientKey(r *http.Request) string {
	address := abuse.ClientIP(r.RemoteAddr, r.Header.Get("X-Forwarded-For"), api.trusted)
	return abuse.RateKey(address)
}

// adminListItemResponse is the metadata-only paginated listing shape. It
// never contains payload content, ciphertext, nonce, storage keys, or verifier.
type adminListItemResponse struct {
	ID            string             `json:"id"`
	PayloadKind   domain.PayloadKind `json:"payload_kind"`
	PrivacyMode   domain.PrivacyMode `json:"privacy_mode"`
	State         string             `json:"state"`
	CreatedAt     string             `json:"created_at"`
	UpdatedAt     string             `json:"updated_at"`
	ExpiresAt     *string            `json:"expires_at"`
	PayloadBytes  int64              `json:"payload_bytes"`
	FileFilename  string             `json:"file_filename,omitempty"`
	FileMediaType string             `json:"file_media_type,omitempty"`
}

func (api *API) listItemFromShare(value share.AdminListItem) adminListItemResponse {
	response := adminListItemResponse{
		ID:            value.ID.String(),
		PayloadKind:   value.PayloadKind,
		PrivacyMode:   value.PrivacyMode,
		CreatedAt:     value.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:     value.UpdatedAt.UTC().Format(time.RFC3339Nano),
		PayloadBytes:  value.PayloadBytes,
		FileFilename:  value.FileFilename,
		FileMediaType: value.FileMediaType,
	}
	if domain.IsExpired(value.ExpiresAt, api.now()) {
		response.State = "expired"
	} else {
		response.State = "active"
	}
	if value.ExpiresAt != nil {
		formatted := value.ExpiresAt.UTC().Format(time.RFC3339Nano)
		response.ExpiresAt = &formatted
	}
	return response
}

type adminShareResponse struct {
	ID            string              `json:"id"`
	PayloadKind   domain.PayloadKind  `json:"payload_kind"`
	PrivacyMode   domain.PrivacyMode  `json:"privacy_mode"`
	State         string              `json:"state"`
	CreatedAt     string              `json:"created_at"`
	UpdatedAt     string              `json:"updated_at"`
	ExpiresAt     *string             `json:"expires_at"`
	Text          *adminTextView      `json:"text,omitempty"`
	EncryptedText *adminEncryptedView `json:"encrypted_text,omitempty"`
	File          *adminFileView      `json:"file,omitempty"`
}

type adminTextView struct {
	Format  domain.TextFormat `json:"format"`
	Content string            `json:"content"`
}

// adminEncryptedView deliberately exposes no ciphertext bytes or keys.
type adminEncryptedView struct {
	Protocol        string `json:"protocol"`
	Nonce           string `json:"nonce"`
	CiphertextBytes int    `json:"ciphertext_bytes"`
	Notice          string `json:"notice"`
}

type adminFileView struct {
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	MediaType   string `json:"media_type"`
	SHA256      string `json:"sha256"`
	DownloadURL string `json:"download_url"`
}

func (api *API) responseFromShare(value share.Share) adminShareResponse {
	response := adminShareResponse{
		ID:          value.ID.String(),
		PayloadKind: value.PayloadKind,
		PrivacyMode: value.PrivacyMode,
		CreatedAt:   value.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:   value.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if domain.IsExpired(value.ExpiresAt, api.now()) {
		response.State = "expired"
	} else {
		response.State = "active"
	}
	if value.ExpiresAt != nil {
		formatted := value.ExpiresAt.UTC().Format(time.RFC3339Nano)
		response.ExpiresAt = &formatted
	}
	if value.Text != nil {
		response.Text = &adminTextView{Format: value.Text.Format, Content: value.Text.Content}
	}
	if value.EncryptedText != nil {
		response.EncryptedText = &adminEncryptedView{
			Protocol:        value.EncryptedText.Protocol,
			Nonce:           base64URL(value.EncryptedText.Nonce),
			CiphertextBytes: len(value.EncryptedText.Ciphertext),
			Notice:          "Client-side encrypted — plaintext unavailable to the server.",
		}
	}
	if value.File != nil {
		response.File = &adminFileView{
			Filename:    value.File.Filename,
			Size:        value.File.Size,
			MediaType:   value.File.MediaType,
			SHA256:      hex.EncodeToString(value.File.SHA256[:]),
			DownloadURL: api.fileOrigin + "/f/" + value.ID.String(),
		}
	}
	return response
}

func parseFilter(r *http.Request) (share.AdminFilter, error) {
	query := r.URL.Query()
	filter := share.AdminFilter{Cursor: query.Get("cursor"), Lifecycle: query.Get("lifecycle"), Sort: query.Get("sort")}
	if limitValue := query.Get("limit"); limitValue != "" {
		limit, err := strconv.Atoi(limitValue)
		if err != nil || limit < 1 || limit > maxPageSize {
			return share.AdminFilter{}, errors.New("invalid limit")
		}
		filter.Limit = limit
	} else {
		filter.Limit = defaultPageSize
	}
	if value := query.Get("kind"); value != "" {
		kind, err := domain.ParsePayloadKind(value)
		if err != nil {
			return share.AdminFilter{}, err
		}
		filter.PayloadKind = kind
	}
	if value := query.Get("privacy"); value != "" {
		privacy, err := domain.ParsePrivacyMode(value)
		if err != nil {
			return share.AdminFilter{}, err
		}
		filter.PrivacyMode = privacy
	}
	if value := query.Get("id"); value != "" {
		id, err := capability.ParseShareID(value)
		if err != nil {
			return share.AdminFilter{}, err
		}
		filter.ExactID = id
	}
	return filter, nil
}

func decodeStrict(r *http.Request, destination any) error {
	limited := io.LimitReader(r.Body, maxAdminBody+1)
	decoder := json.NewDecoder(limited)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("unexpected trailing data")
	}
	return nil
}

func (api *API) writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		api.log.Error("write admin response failed", "error", err)
	}
}

func (api *API) writeError(w http.ResponseWriter, status int, code, message string) {
	api.writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func (api *API) internalError(w http.ResponseWriter, operation string, err error) {
	api.log.Error("admin request failed", "operation", operation, "error", err)
	api.writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func base64URL(value []byte) string {
	return base64.RawURLEncoding.EncodeToString(value)
}
