package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/database"
	"github.com/DejavuMoe/uPaste/internal/objectstore"
	"github.com/DejavuMoe/uPaste/internal/share"
)

type apiTestEnv struct {
	t       *testing.T
	db      *sql.DB
	store   *objectstore.Local
	shares  *share.Service
	log     *slog.Logger
	handler http.Handler
	now     *time.Time
	logs    *bytes.Buffer
}

func newAPITestEnv(t *testing.T) *apiTestEnv {
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
	now := time.Date(2026, 9, 14, 3, 0, 0, 0, time.UTC)
	logs := new(bytes.Buffer)
	log := slog.New(slog.NewTextHandler(logs, nil))
	service := share.NewWithStore(db, store, func() time.Time { return now })
	env := &apiTestEnv{t: t, db: db, store: store, shares: service, log: log, handler: New(service, "https://files.example.test", log), now: &now, logs: logs}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return env
}

func (env *apiTestEnv) request(method, path string, body []byte) *httptest.ResponseRecorder {
	env.t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	return env.serve(request)
}

func (env *apiTestEnv) serve(request *http.Request) *httptest.ResponseRecorder {
	env.t.Helper()
	response := httptest.NewRecorder()
	env.handler.ServeHTTP(response, request)
	return response
}

func (env *apiTestEnv) create(format, content string, expiresAt any) createResponse {
	env.t.Helper()
	body, err := json.Marshal(map[string]any{
		"payload_kind": "TEXT",
		"privacy_mode": "STANDARD",
		"text":         map[string]any{"format": format, "content": content},
		"expires_at":   expiresAt,
	})
	if err != nil {
		env.t.Fatal(err)
	}
	response := env.request(http.MethodPost, "/api/v1/shares", body)
	if response.Code != http.StatusCreated {
		env.t.Fatalf("create status = %d, want 201; body=%s", response.Code, response.Body.String())
	}
	var decoded createResponse
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		env.t.Fatal(err)
	}
	return decoded
}

func bearer(request *http.Request, token string) {
	request.Header.Set("Authorization", "Bearer "+token)
}

func assertAPIHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Error("API response is not no-store")
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("API response is missing nosniff")
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("API response has a CORS allow-origin header")
	}
}

func assertRawHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	want := map[string]string{
		"Content-Type":            "text/plain; charset=utf-8",
		"X-Content-Type-Options":  "nosniff",
		"Cache-Control":           "no-store",
		"Referrer-Policy":         "no-referrer",
		"X-Frame-Options":         "DENY",
		"Content-Security-Policy": "default-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'; sandbox",
	}
	for name, value := range want {
		if got := response.Header().Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("raw response has a CORS allow-origin header")
	}
}

func TestCreateStandardText(t *testing.T) {
	env := newAPITestEnv(t)
	for _, format := range []string{"PLAIN", "SOURCE", "MARKDOWN"} {
		t.Run(format, func(t *testing.T) {
			response := env.create(format, "  content\n", nil)
			if response.Share.PayloadKind != "TEXT" || response.Share.PrivacyMode != "STANDARD" || response.Share.Text == nil || string(response.Share.Text.Format) != format || response.Share.Text.Content != "  content\n" || response.Share.EncryptedText != nil {
				t.Fatal("create response changed Share data")
			}
			if response.Share.ExpiresAt != nil {
				t.Fatal("explicit null expiration was not preserved")
			}
			if response.Share.CreatedAt != "2026-09-14T03:00:00Z" || response.Share.UpdatedAt != response.Share.CreatedAt {
				t.Fatal("create timestamps are incorrect")
			}
			if _, err := capability.ParseShareID(response.Share.ID); err != nil {
				t.Fatal("response Share ID is invalid")
			}
			if _, err := capability.ParseOwnerToken(response.OwnerToken); err != nil {
				t.Fatal("response owner token is invalid")
			}
		})
	}

	future := env.now.Add(time.Hour).Add(987654 * time.Nanosecond)
	response := env.create("PLAIN", "future", future.Format(time.RFC3339Nano))
	if response.Share.ExpiresAt == nil || *response.Share.ExpiresAt != "2026-09-14T04:00:00Z" {
		t.Fatalf("normalized expiration = %v", response.Share.ExpiresAt)
	}

	body, _ := json.Marshal(map[string]any{
		"payload_kind": "TEXT", "privacy_mode": "STANDARD",
		"text": map[string]any{"format": "PLAIN", "content": "location"}, "expires_at": nil,
	})
	recorded := env.request(http.MethodPost, "/api/v1/shares", body)
	assertAPIHeaders(t, recorded)
	var created createResponse
	if err := json.Unmarshal(recorded.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if recorded.Header().Get("Location") != "/api/v1/shares/"+created.Share.ID {
		t.Fatal("Location is incorrect")
	}
	if strings.Contains(recorded.Header().Get("Location"), created.OwnerToken) {
		t.Fatal("Location contains owner token")
	}
	if strings.Contains(recorded.Body.String(), "owner_token_verifier") {
		t.Fatal("create response exposes owner verifier")
	}
	parsedToken, err := capability.ParseOwnerToken(created.OwnerToken)
	if err != nil {
		t.Fatal("created owner token did not parse")
	}
	var storedVerifier []byte
	if err := env.db.QueryRow("SELECT owner_token_verifier FROM shares WHERE id = ?", created.Share.ID).Scan(&storedVerifier); err != nil {
		t.Fatal(err)
	}
	expectedVerifier := parsedToken.Verifier()
	if !bytes.Equal(storedVerifier, expectedVerifier[:]) {
		t.Fatal("database did not store the expected verifier")
	}
}

func TestCreateValidation(t *testing.T) {
	env := newAPITestEnv(t)
	valid := func() map[string]any {
		return map[string]any{
			"payload_kind": "TEXT", "privacy_mode": "STANDARD",
			"text": map[string]any{"format": "PLAIN", "content": "content"}, "expires_at": nil,
		}
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
		status int
		code   string
	}{
		{"missing payload kind", func(v map[string]any) { delete(v, "payload_kind") }, 400, "invalid_request"},
		{"missing privacy", func(v map[string]any) { delete(v, "privacy_mode") }, 400, "invalid_request"},
		{"missing text", func(v map[string]any) { delete(v, "text") }, 400, "invalid_request"},
		{"missing expiration", func(v map[string]any) { delete(v, "expires_at") }, 400, "invalid_request"},
		{"missing text format", func(v map[string]any) { delete(v["text"].(map[string]any), "format") }, 400, "invalid_request"},
		{"missing text content", func(v map[string]any) { delete(v["text"].(map[string]any), "content") }, 400, "invalid_request"},
		{"invalid payload kind", func(v map[string]any) { v["payload_kind"] = "OTHER" }, 400, "invalid_request"},
		{"invalid privacy", func(v map[string]any) { v["privacy_mode"] = "OTHER" }, 400, "invalid_request"},
		{"invalid format", func(v map[string]any) { v["text"].(map[string]any)["format"] = "OTHER" }, 400, "invalid_request"},
		{"file", func(v map[string]any) { v["payload_kind"] = "FILE" }, 422, "unsupported_share_type"},
		{"missing encrypted text", func(v map[string]any) { v["privacy_mode"] = "ENCRYPTED"; delete(v, "text") }, 400, "invalid_request"},
		{"encrypted file", func(v map[string]any) { v["payload_kind"] = "FILE"; v["privacy_mode"] = "ENCRYPTED" }, 422, "unsupported_share_type"},
		{"unknown field", func(v map[string]any) { v["unknown"] = true }, 400, "invalid_request"},
		{"unknown text field", func(v map[string]any) { v["text"].(map[string]any)["unknown"] = true }, 400, "invalid_request"},
		{"empty content", func(v map[string]any) { v["text"].(map[string]any)["content"] = "" }, 400, "invalid_request"},
		{"oversized content", func(v map[string]any) {
			v["text"].(map[string]any)["content"] = strings.Repeat("x", share.MaxTextBytes+1)
		}, 413, "request_too_large"},
		{"past expiration", func(v map[string]any) { v["expires_at"] = env.now.Add(-time.Second).Format(time.RFC3339Nano) }, 400, "invalid_request"},
		{"expiration at now", func(v map[string]any) { v["expires_at"] = env.now.Format(time.RFC3339Nano) }, 400, "invalid_request"},
		{"expiration future before millisecond normalization", func(v map[string]any) { v["expires_at"] = env.now.Add(500 * time.Microsecond).Format(time.RFC3339Nano) }, 400, "invalid_request"},
		{"invalid expiration", func(v map[string]any) { v["expires_at"] = "tomorrow" }, 400, "invalid_request"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := valid()
			test.mutate(value)
			body, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			response := env.request(http.MethodPost, "/api/v1/shares", body)
			assertError(t, response, test.status, test.code)
		})
	}

	for name, body := range map[string]string{
		"malformed": `{`,
		"trailing":  `{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"x"},"expires_at":null} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			assertError(t, env.request(http.MethodPost, "/api/v1/shares", []byte(body)), 400, "invalid_request")
		})
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "text/plain")
	assertError(t, env.serve(request), 415, "unsupported_media_type")
	request = httptest.NewRequest(http.MethodPost, "/api/v1/shares", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Content-Encoding", "gzip")
	assertError(t, env.serve(request), 415, "unsupported_media_type")
	assertError(t, env.request(http.MethodPost, "/api/v1/shares", bytes.Repeat([]byte("x"), maxJSONBytes+1)), 413, "request_too_large")
}

func TestReadAndRaw(t *testing.T) {
	env := newAPITestEnv(t)
	for _, test := range []struct{ format, content string }{
		{"PLAIN", "plain"},
		{"SOURCE", "package main"},
		{"MARKDOWN", "# heading"},
		{"PLAIN", "<script>alert(1)</script>"},
	} {
		created := env.create(test.format, test.content, nil)
		read := env.request(http.MethodGet, "/api/v1/shares/"+created.Share.ID, nil)
		if read.Code != http.StatusOK {
			t.Fatalf("read status = %d", read.Code)
		}
		assertAPIHeaders(t, read)
		if strings.Contains(read.Body.String(), "owner_token") || strings.Contains(read.Body.String(), "verifier") {
			t.Fatal("read leaked owner material")
		}
		var decoded shareResponse
		if err := json.Unmarshal(read.Body.Bytes(), &decoded); err != nil {
			t.Fatal(err)
		}
		if decoded.Share.Text.Content != test.content || string(decoded.Share.Text.Format) != test.format {
			t.Fatal("JSON read changed stored text")
		}
		raw := env.request(http.MethodGet, "/raw/"+created.Share.ID, nil)
		if raw.Code != http.StatusOK || raw.Body.String() != test.content {
			t.Fatalf("raw response changed content: status=%d", raw.Code)
		}
		assertRawHeaders(t, raw)
	}

	assertError(t, env.request(http.MethodGet, "/api/v1/shares/not-an-id", nil), 404, "not_found")
	missingID, err := capability.GenerateShareID()
	if err != nil {
		t.Fatal("generate missing Share ID")
	}
	assertError(t, env.request(http.MethodGet, "/api/v1/shares/"+missingID.String(), nil), 404, "not_found")
	assertError(t, env.request(http.MethodGet, "/api/v1/shares/", nil), 404, "not_found")
	missingRaw := env.request(http.MethodGet, "/raw/"+missingID.String(), nil)
	if missingRaw.Code != 404 {
		t.Fatalf("missing raw Share status = %d", missingRaw.Code)
	}
	assertRawHeaders(t, missingRaw)
	missingRaw = env.request(http.MethodGet, "/raw/not-an-id", nil)
	if missingRaw.Code != 404 {
		t.Fatalf("malformed raw ID status = %d", missingRaw.Code)
	}
	assertRawHeaders(t, missingRaw)
	missingRaw = env.request(http.MethodGet, "/raw", nil)
	if missingRaw.Code != 404 {
		t.Fatalf("missing raw ID status = %d", missingRaw.Code)
	}
	assertRawHeaders(t, missingRaw)
}

func TestExpirationBlocksReadAndRaw(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(time.Hour)
	created := env.create("PLAIN", "must-not-leak", expires.Format(time.RFC3339Nano))
	*env.now = expires
	read := env.request(http.MethodGet, "/api/v1/shares/"+created.Share.ID, nil)
	assertError(t, read, 410, "expired")
	if strings.Contains(read.Body.String(), "must-not-leak") {
		t.Fatal("expired API response leaked content")
	}
	raw := env.request(http.MethodGet, "/raw/"+created.Share.ID, nil)
	if raw.Code != 410 || strings.Contains(raw.Body.String(), "must-not-leak") {
		t.Fatal("expired raw response leaked content")
	}
	assertRawHeaders(t, raw)
}

func TestMethodContracts(t *testing.T) {
	env := newAPITestEnv(t)
	collection := env.request(http.MethodGet, "/api/v1/shares", nil)
	assertError(t, collection, 405, "invalid_request")
	if collection.Header().Get("Allow") != "POST" {
		t.Fatal("collection Allow header is incorrect")
	}
	item := env.request(http.MethodPut, "/api/v1/shares/not-an-id", nil)
	assertError(t, item, 405, "invalid_request")
	if item.Header().Get("Allow") != "GET, PATCH, DELETE" {
		t.Fatal("item Allow header is incorrect")
	}
	raw := env.request(http.MethodPost, "/raw/not-an-id", nil)
	if raw.Code != 405 || raw.Header().Get("Allow") != "GET" {
		t.Fatal("raw method contract is incorrect")
	}
	assertRawHeaders(t, raw)
	assertError(t, env.request(http.MethodGet, "/api/v1/unknown", nil), 404, "not_found")
}

func assertError(t *testing.T, response *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status = %d, want %d; body=%s", response.Code, status, response.Body.String())
	}
	var decoded errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode error response: %v; body=%s", err, response.Body.String())
	}
	if decoded.Error.Code != code {
		t.Fatalf("error code = %q, want %q", decoded.Error.Code, code)
	}
	assertAPIHeaders(t, response)
}
