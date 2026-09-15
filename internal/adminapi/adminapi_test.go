package adminapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/admin"
	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/domain"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
	"github.com/DejavuMoe/uPaste/internal/share"
)

type fakeCleaner struct{ purged int }

func (cleaner *fakeCleaner) CleanupExpired(context.Context, time.Time) (int, error) {
	return cleaner.purged, nil
}

type env struct {
	t       *testing.T
	service *share.Service
	store   *objectstore.Local
	manager *admin.Manager
	handler http.Handler
	token   admin.Token
	cleaner *fakeCleaner
}

func newEnv(t *testing.T) *env {
	t.Helper()
	dataDir := t.TempDir()
	db, err := database.Open(context.Background(), dataDir)
	if err != nil {
		t.Fatal(err)
	}
	store, err := objectstore.OpenLocal(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	token, err := admin.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now
	service := share.NewWithStore(db, store, now)
	manager := admin.NewManager(token.Verifier(), admin.Options{SecureCookie: false, SessionTTL: 8 * time.Hour, Now: now})
	cleaner := &fakeCleaner{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(service, manager, cleaner, "https://files.example.test", nil, log, now)
	t.Cleanup(func() { _ = db.Close() })
	return &env{t: t, service: service, store: store, manager: manager, handler: handler, token: token, cleaner: cleaner}
}

func (env *env) serve(request *http.Request) *httptest.ResponseRecorder {
	env.t.Helper()
	recorder := httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, request)
	return recorder
}

func (env *env) login() (cookie *http.Cookie, csrf string) {
	env.t.Helper()
	body := strings.NewReader(`{"token":"` + env.token.Reveal() + `"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session", body)
	request.Header.Set("Content-Type", "application/json")
	response := env.serve(request)
	if response.Code != http.StatusOK {
		env.t.Fatalf("login status = %d; body=%s", response.Code, response.Body.String())
	}
	var decoded struct {
		CSRF string `json:"csrf"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		env.t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		env.t.Fatalf("login cookies = %d", len(cookies))
	}
	return cookies[0], decoded.CSRF
}

func (env *env) request(method, path string, cookie *http.Cookie, csrf string, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if csrf != "" {
		request.Header.Set(admin.CSRFHeaderName, csrf)
	}
	return env.serve(request)
}

func TestAdminLoginCookieSessionAndCSRF(t *testing.T) {
	env := newEnv(t)

	// Wrong token is generic.
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session", strings.NewReader(`{"token":"up_a1_`+strings.Repeat("A", 43)+`"}`))
	request.Header.Set("Content-Type", "application/json")
	response := env.serve(request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), env.token.Reveal()) {
		t.Fatalf("wrong login status/body = %d/%s", response.Code, response.Body.String())
	}

	cookie, csrf := env.login()
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatalf("session cookie attributes = %+v", cookie)
	}
	if strings.Contains(cookie.Value, env.token.Reveal()) {
		t.Fatal("admin token became a session cookie")
	}

	// Session endpoint recovers CSRF via the HttpOnly cookie.
	sessionResponse := env.request(http.MethodGet, "/api/v1/admin/session", cookie, "", "")
	if sessionResponse.Code != http.StatusOK || !strings.Contains(sessionResponse.Body.String(), csrf) {
		t.Fatalf("session recovery = %d/%s", sessionResponse.Code, sessionResponse.Body.String())
	}

	// State-changing request requires CSRF.
	if response := env.request(http.MethodDelete, "/api/v1/admin/session", cookie, "", ""); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", response.Code)
	}
	if response := env.request(http.MethodDelete, "/api/v1/admin/session", cookie, csrf+"x", ""); response.Code != http.StatusForbidden {
		t.Fatalf("wrong CSRF status = %d", response.Code)
	}
	if response := env.request(http.MethodDelete, "/api/v1/admin/session", cookie, csrf, ""); response.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d", response.Code)
	}
	if response := env.request(http.MethodGet, "/api/v1/admin/session", cookie, "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("post-logout session status = %d", response.Code)
	}
}

func TestAdminListingInspectionAndDeleteGovernance(t *testing.T) {
	env := newEnv(t)
	ctx := context.Background()
	text, _, err := env.service.Create(ctx, share.CreateInput{Text: share.Text{Format: domain.TextPlain, Content: "admin visible text"}})
	if err != nil {
		t.Fatal(err)
	}
	encrypted, _, err := env.service.CreateEncrypted(ctx, share.EncryptedCreateInput{EncryptedText: share.EncryptedText{
		Protocol: share.EncryptedTextProtocolV1, Nonce: make([]byte, 12), Ciphertext: make([]byte, 19),
	}})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := env.service.StageFile(ctx, strings.NewReader("admin file"))
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := env.service.CreateFile(ctx, "admin.bin", staged, nil)
	if err != nil {
		t.Fatal(err)
	}
	cookie, csrf := env.login()

	// Summary.
	summary := env.request(http.MethodGet, "/api/v1/admin/summary", cookie, "", "")
	if summary.Code != http.StatusOK {
		t.Fatalf("summary status = %d", summary.Code)
	}
	var summaryBody struct {
		Summary share.AdminSummary `json:"summary"`
	}
	if err := json.Unmarshal(summary.Body.Bytes(), &summaryBody); err != nil {
		t.Fatal(err)
	}
	if summaryBody.Summary.Text != 2 || summaryBody.Summary.File != 1 || summaryBody.Summary.Encrypted != 1 || summaryBody.Summary.FileBytes != int64(len("admin file")) {
		t.Fatalf("summary = %+v", summaryBody.Summary)
	}

	// List and detail.
	list := env.request(http.MethodGet, "/api/v1/admin/shares?limit=10", cookie, "", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}
	if strings.Contains(list.Body.String(), "owner_token") || strings.Contains(list.Body.String(), "verifier") {
		t.Fatal("admin list leaked owner material")
	}
	detail := env.request(http.MethodGet, "/api/v1/admin/shares/"+text.ID.String(), cookie, "", "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "admin visible text") {
		t.Fatalf("text detail = %d/%s", detail.Code, detail.Body.String())
	}
	encryptedDetail := env.request(http.MethodGet, "/api/v1/admin/shares/"+encrypted.ID.String(), cookie, "", "")
	if encryptedDetail.Code != http.StatusOK {
		t.Fatalf("encrypted detail status = %d", encryptedDetail.Code)
	}
	encryptedBody := encryptedDetail.Body.String()
	if strings.Contains(encryptedBody, "plaintext unavailable") == false || strings.Contains(encryptedBody, "ciphertext\"") {
		t.Fatalf("encrypted admin view = %s", encryptedBody)
	}
	fileDetail := env.request(http.MethodGet, "/api/v1/admin/shares/"+file.ID.String(), cookie, "", "")
	if fileDetail.Code != http.StatusOK || !strings.Contains(fileDetail.Body.String(), "https://files.example.test/f/"+file.ID.String()) {
		t.Fatalf("file detail = %d/%s", fileDetail.Code, fileDetail.Body.String())
	}

	// No content-edit endpoint exists.
	if response := env.request(http.MethodPatch, "/api/v1/admin/shares/"+text.ID.String(), cookie, csrf, `{"text":{"content":"edited"}}`); response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("admin PATCH status = %d", response.Code)
	}

	// Single delete removes metadata and File object.
	if response := env.request(http.MethodDelete, "/api/v1/admin/shares/"+file.ID.String(), cookie, csrf, ""); response.Code != http.StatusNoContent {
		t.Fatalf("file delete status = %d", response.Code)
	}
	if _, err := env.service.AdminGet(ctx, file.ID); err == nil {
		t.Fatal("deleted File Share remained")
	}
	if _, _, err := env.service.OpenFile(ctx, file.ID); err == nil {
		t.Fatal("deleted File object remained downloadable")
	}

	// Bulk delete is bounded and removes selected Shares.
	bulkBody := `{"ids":["` + text.ID.String() + `","` + encrypted.ID.String() + `"]}`
	bulk := env.request(http.MethodPost, "/api/v1/admin/shares/bulk-delete", cookie, csrf, bulkBody)
	if bulk.Code != http.StatusOK || !strings.Contains(bulk.Body.String(), `"deleted":2`) {
		t.Fatalf("bulk delete = %d/%s", bulk.Code, bulk.Body.String())
	}

	// Cleanup reuses the cleaner interface.
	env.cleaner.purged = 3
	cleanup := env.request(http.MethodPost, "/api/v1/admin/cleanup/expired", cookie, csrf, "")
	if cleanup.Code != http.StatusOK || !strings.Contains(cleanup.Body.String(), `"purged":3`) {
		t.Fatalf("cleanup = %d/%s", cleanup.Code, cleanup.Body.String())
	}
}

func TestAdminLoginBruteForceRateLimit(t *testing.T) {
	env := newEnv(t)
	body := `{"token":"up_a1_` + strings.Repeat("A", 43) + `"}`
	for attempt := 0; attempt < 5; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		if response := env.serve(request); response.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status = %d", attempt, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/session", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := env.serve(request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "" {
		t.Fatalf("rate limit status/retry = %d/%q", response.Code, response.Header().Get("Retry-After"))
	}
}

func TestAdminListIsMetadataOnlyWithLargePayloads(t *testing.T) {
	env := newEnv(t)
	ctx := context.Background()
	largeText := "large-text-secret-" + strings.Repeat("x", 300*1024)
	text, _, err := env.service.Create(ctx, share.CreateInput{Text: share.Text{Format: domain.TextPlain, Content: largeText}})
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := make([]byte, 300*1024)
	for i := range ciphertext {
		ciphertext[i] = byte(i)
	}
	encrypted, _, err := env.service.CreateEncrypted(ctx, share.EncryptedCreateInput{EncryptedText: share.EncryptedText{
		Protocol: share.EncryptedTextProtocolV1, Nonce: make([]byte, 12), Ciphertext: ciphertext,
	}})
	if err != nil {
		t.Fatal(err)
	}
	staged, err := env.service.StageFile(ctx, strings.NewReader(strings.Repeat("f", 200*1024)))
	if err != nil {
		t.Fatal(err)
	}
	file, _, err := env.service.CreateFile(ctx, "large-admin.bin", staged, nil)
	if err != nil {
		t.Fatal(err)
	}

	cookie, _ := env.login()
	list := env.request(http.MethodGet, "/api/v1/admin/shares?limit=10", cookie, "", "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d; body=%s", list.Code, list.Body.String())
	}
	raw := list.Body.String()
	for _, forbidden := range []string{
		"large-text-secret-",
		base64.StdEncoding.EncodeToString(ciphertext),
		base64.RawURLEncoding.EncodeToString(ciphertext),
		`"text"`, `"encrypted_text"`, `"file"`, `"nonce"`, `"ciphertext"`, `"storage_key"`, "owner_token", "verifier",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("metadata-only list leaked %q: %s", forbidden, raw[:min(len(raw), 400)])
		}
	}
	var listBody struct {
		Shares []struct {
			ID            string `json:"id"`
			PayloadKind   string `json:"payload_kind"`
			PrivacyMode   string `json:"privacy_mode"`
			State         string `json:"state"`
			PayloadBytes  int64  `json:"payload_bytes"`
			FileFilename  string `json:"file_filename"`
			FileMediaType string `json:"file_media_type"`
		} `json:"shares"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listBody); err != nil {
		t.Fatal(err)
	}
	if len(listBody.Shares) != 3 {
		t.Fatalf("list shares = %d", len(listBody.Shares))
	}
	bytesByID := map[string]int64{}
	for _, item := range listBody.Shares {
		bytesByID[item.ID] = item.PayloadBytes
	}
	if bytesByID[text.ID.String()] != int64(len(largeText)) ||
		bytesByID[encrypted.ID.String()] != int64(len(ciphertext)) ||
		bytesByID[file.ID.String()] != int64(200*1024) {
		t.Fatalf("payload bytes = %+v", bytesByID)
	}

	// Detail remains explicit and on demand.
	detail := env.request(http.MethodGet, "/api/v1/admin/shares/"+text.ID.String(), cookie, "", "")
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), "large-text-secret-") {
		t.Fatalf("text detail status/content = %d", detail.Code)
	}
	encryptedDetail := env.request(http.MethodGet, "/api/v1/admin/shares/"+encrypted.ID.String(), cookie, "", "")
	if encryptedDetail.Code != http.StatusOK {
		t.Fatalf("encrypted detail status = %d", encryptedDetail.Code)
	}
	if strings.Contains(encryptedDetail.Body.String(), base64.RawURLEncoding.EncodeToString(ciphertext)) {
		t.Fatal("encrypted detail exposed ciphertext bytes")
	}
	if !strings.Contains(encryptedDetail.Body.String(), "plaintext unavailable") {
		t.Fatal("encrypted detail missing unavailable notice")
	}

	exact := env.request(http.MethodGet, "/api/v1/admin/shares?id="+encrypted.ID.String(), cookie, "", "")
	if exact.Code != http.StatusOK || !strings.Contains(exact.Body.String(), encrypted.ID.String()) {
		t.Fatalf("exact metadata list status = %d", exact.Code)
	}
	if strings.Contains(exact.Body.String(), base64.RawURLEncoding.EncodeToString(ciphertext)) || strings.Contains(exact.Body.String(), `"nonce"`) {
		t.Fatal("exact metadata list leaked encrypted payload data")
	}
}
