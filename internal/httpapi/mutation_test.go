package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/share"
)

func TestOwnerAuthorization(t *testing.T) {
	env := newAPITestEnv(t)
	created := env.create("PLAIN", "content", nil)
	path := "/api/v1/shares/" + created.Share.ID
	patch := []byte(`{"expires_at":null}`)

	tests := []struct {
		name    string
		request func() *http.Request
	}{
		{"missing", func() *http.Request {
			return httptest.NewRequest(http.MethodPatch, path, strings.NewReader(string(patch)))
		}},
		{"malformed scheme", func() *http.Request {
			r := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(string(patch)))
			r.Header.Set("Authorization", "Basic ignored")
			return r
		}},
		{"malformed token", func() *http.Request {
			r := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(string(patch)))
			bearer(r, "invalid-owner-token")
			return r
		}},
		{"query token ignored", func() *http.Request {
			return httptest.NewRequest(http.MethodPatch, path+"?owner_token="+url.QueryEscape(created.OwnerToken), strings.NewReader(string(patch)))
		}},
		{"multiple headers", func() *http.Request {
			r := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(string(patch)))
			r.Header.Add("Authorization", "Bearer "+created.OwnerToken)
			r.Header.Add("Authorization", "Bearer "+created.OwnerToken)
			return r
		}},
	}
	wrong, err := capability.GenerateOwnerToken()
	if err != nil {
		t.Fatal("generate alternate owner token")
	}
	tests = append(tests, struct {
		name    string
		request func() *http.Request
	}{"wrong valid token", func() *http.Request {
		r := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(string(patch)))
		bearer(r, wrong.String())
		return r
	}})

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := test.request()
			request.Header.Set("Content-Type", "application/json")
			response := env.serve(request)
			assertError(t, response, 401, "unauthorized")
			if response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("WWW-Authenticate is missing")
			}
		})
	}

	request := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(string(patch)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "bEaReR "+created.OwnerToken)
	if response := env.serve(request); response.Code != http.StatusOK {
		t.Fatalf("case-insensitive Bearer status = %d", response.Code)
	}
}

func TestPatch(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(2 * time.Hour)
	created := env.create("PLAIN", "original", expires.Format(time.RFC3339Nano))
	path := "/api/v1/shares/" + created.Share.ID
	*env.now = env.now.Add(time.Hour)

	response := authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{
		"text": map[string]any{"format": "SOURCE", "content": "package main"},
	})
	if response.Code != http.StatusOK {
		t.Fatalf("text patch status = %d; body=%s", response.Code, response.Body.String())
	}
	var textPatched shareResponse
	if err := json.Unmarshal(response.Body.Bytes(), &textPatched); err != nil {
		t.Fatal(err)
	}
	if textPatched.Share.Text.Format != "SOURCE" || textPatched.Share.Text.Content != "package main" {
		t.Fatal("text patch was not applied")
	}
	if textPatched.Share.ExpiresAt == nil || *textPatched.Share.ExpiresAt != expires.Format(time.RFC3339Nano) {
		t.Fatal("omitted expiration changed")
	}
	if textPatched.Share.UpdatedAt != env.now.Format(time.RFC3339Nano) {
		t.Fatal("updated_at did not use injected clock")
	}
	if strings.Contains(response.Body.String(), "owner_token") || strings.Contains(response.Body.String(), "verifier") {
		t.Fatal("PATCH leaked owner material")
	}

	future := env.now.Add(2 * time.Hour)
	response = authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"expires_at": future.Format(time.RFC3339Nano)})
	if response.Code != http.StatusOK {
		t.Fatalf("expiration patch status = %d", response.Code)
	}
	response = authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{
		"text":       map[string]any{"format": "MARKDOWN", "content": "# both"},
		"expires_at": future.Format(time.RFC3339Nano),
	})
	if response.Code != http.StatusOK {
		t.Fatalf("combined patch status = %d", response.Code)
	}
	response = authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"expires_at": nil})
	if response.Code != http.StatusOK {
		t.Fatalf("clear expiration status = %d", response.Code)
	}
	var cleared shareResponse
	if err := json.Unmarshal(response.Body.Bytes(), &cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.Share.ExpiresAt != nil {
		t.Fatal("explicit null did not clear expiration")
	}

	for _, test := range []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"past expiration", map[string]any{"expires_at": env.now.Add(-time.Second).Format(time.RFC3339Nano)}, 400, "invalid_request"},
		{"empty", map[string]any{}, 400, "invalid_request"},
		{"immutable field", map[string]any{"privacy_mode": "STANDARD"}, 400, "invalid_request"},
		{"null text", map[string]any{"text": nil}, 400, "invalid_request"},
		{"missing text content", map[string]any{"text": map[string]any{"format": "PLAIN"}}, 400, "invalid_request"},
		{"unknown text field", map[string]any{"text": map[string]any{"format": "PLAIN", "content": "x", "unknown": true}}, 400, "invalid_request"},
		{"oversized text", map[string]any{"text": map[string]any{"format": "PLAIN", "content": strings.Repeat("x", share.MaxTextBytes+1)}}, 413, "request_too_large"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assertError(t, authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, test.body), test.status, test.code)
		})
	}
	for name, body := range map[string]string{"malformed": "{", "trailing": `{"expires_at":null} {}`} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			bearer(request, created.OwnerToken)
			assertError(t, env.serve(request), 400, "invalid_request")
		})
	}
	request := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"expires_at":null}`))
	request.Header.Set("Content-Type", "text/plain")
	bearer(request, created.OwnerToken)
	assertError(t, env.serve(request), 415, "unsupported_media_type")
}

func TestExpiredShareCannotBeRevived(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(time.Hour)
	created := env.create("PLAIN", "expired content", expires.Format(time.RFC3339Nano))
	*env.now = expires
	future := expires.Add(time.Hour)
	response := authorizedJSON(env, http.MethodPatch, "/api/v1/shares/"+created.Share.ID, created.OwnerToken, map[string]any{"expires_at": future.Format(time.RFC3339Nano)})
	assertError(t, response, 410, "expired")
	var stored int64
	if err := env.db.QueryRow("SELECT expires_at FROM shares WHERE id = ?", created.Share.ID).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored != expires.UnixMilli() {
		t.Fatal("expired Share was changed")
	}
}

func TestDelete(t *testing.T) {
	env := newAPITestEnv(t)
	created := env.create("MARKDOWN", "delete me", nil)
	path := "/api/v1/shares/" + created.Share.ID

	unauthorized := httptest.NewRequest(http.MethodDelete, path, nil)
	bearer(unauthorized, "invalid-owner-token")
	assertError(t, env.serve(unauthorized), 401, "unauthorized")

	request := httptest.NewRequest(http.MethodDelete, path, nil)
	bearer(request, created.OwnerToken)
	response := env.serve(request)
	if response.Code != http.StatusNoContent || response.Body.Len() != 0 {
		t.Fatalf("DELETE status/body = %d/%d, want 204/0", response.Code, response.Body.Len())
	}
	assertAPIHeaders(t, response)
	for table, idColumn := range map[string]string{"shares": "id", "standard_text_payloads": "share_id"} {
		var count int
		if err := env.db.QueryRow("SELECT count(*) FROM "+table+" WHERE "+idColumn+" = ?", created.Share.ID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Fatalf("%s row remains after DELETE", table)
		}
	}
	assertError(t, env.request(http.MethodGet, path, nil), 404, "not_found")
	if raw := env.request(http.MethodGet, "/raw/"+created.Share.ID, nil); raw.Code != 404 {
		t.Fatalf("raw after delete status = %d", raw.Code)
	}
	request = httptest.NewRequest(http.MethodDelete, path, nil)
	bearer(request, created.OwnerToken)
	assertError(t, env.serve(request), 404, "not_found")
}

func TestExpiredDeleteLeavesRows(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(time.Hour)
	created := env.create("PLAIN", "expired", expires.Format(time.RFC3339Nano))
	*env.now = expires
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/"+created.Share.ID, nil)
	bearer(request, created.OwnerToken)
	assertError(t, env.serve(request), 410, "expired")
	var count int
	if err := env.db.QueryRow("SELECT count(*) FROM shares WHERE id = ?", created.Share.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("expired DELETE removed the Share")
	}
}

func TestOwnerTokenDoesNotLeakToLogsOrLaterResponses(t *testing.T) {
	env := newAPITestEnv(t)
	created := env.create("PLAIN", "content", nil)
	path := "/api/v1/shares/" + created.Share.ID
	invalid := "invalid-owner-token"
	request := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(`{"expires_at":null}`))
	request.Header.Set("Content-Type", "application/json")
	bearer(request, invalid)
	response := env.serve(request)
	assertError(t, response, 401, "unauthorized")
	read := env.request(http.MethodGet, path, nil)
	if strings.Contains(read.Body.String(), created.OwnerToken) || strings.Contains(read.Body.String(), "owner_token") {
		t.Fatal("read response leaked owner token")
	}
	if strings.Contains(env.logs.String(), created.OwnerToken) || strings.Contains(env.logs.String(), invalid) {
		t.Fatal("logs leaked an owner token candidate")
	}
	if strings.Contains(response.Body.String(), invalid) {
		t.Fatal("error response echoed owner token candidate")
	}
}

func authorizedJSON(env *apiTestEnv, method, path, token string, value map[string]any) *httptest.ResponseRecorder {
	env.t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		env.t.Fatal(err)
	}
	request := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	bearer(request, token)
	return env.serve(request)
}
