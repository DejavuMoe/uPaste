package httpapi

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestExpirationExactBoundaryAcrossHTTPRoutes(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(time.Hour)
	created := env.create("PLAIN", "boundary secret", expires.Format(time.RFC3339Nano))
	path := "/api/v1/shares/" + created.Share.ID

	// Strictly before the boundary the Share is active.
	*env.now = expires.Add(-time.Millisecond)
	if response := env.request(http.MethodGet, path, nil); response.Code != http.StatusOK {
		t.Fatalf("GET before boundary status = %d", response.Code)
	}
	if response := env.request(http.MethodGet, "/raw/"+created.Share.ID, nil); response.Code != http.StatusOK {
		t.Fatalf("raw before boundary status = %d", response.Code)
	}

	// Exactly at the boundary the Share is expired for every route.
	*env.now = expires
	assertError(t, env.request(http.MethodGet, path, nil), http.StatusGone, "expired")
	raw := env.request(http.MethodGet, "/raw/"+created.Share.ID, nil)
	if raw.Code != http.StatusGone {
		t.Fatalf("raw at boundary status = %d", raw.Code)
	}
	assertRawHeaders(t, raw)
	assertError(t, authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"expires_at": nil}), http.StatusGone, "expired")
	assertError(t, authorizedJSON(env, http.MethodDelete, path, created.OwnerToken, map[string]any{}), http.StatusGone, "expired")
}

func TestFileExpirationExactBoundary(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(time.Hour)
	response := env.serve(multipartRequest(t, `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":"`+expires.Format(time.RFC3339Nano)+`"}`, "boundary.bin", []byte("boundary"), false))
	if response.Code != http.StatusCreated {
		t.Fatalf("File create status = %d; body=%s", response.Code, response.Body.String())
	}
	var created createResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/shares/" + created.Share.ID

	*env.now = expires.Add(-time.Millisecond)
	if response := env.request(http.MethodGet, path, nil); response.Code != http.StatusOK {
		t.Fatalf("File GET before boundary status = %d", response.Code)
	}

	*env.now = expires
	assertError(t, env.request(http.MethodGet, path, nil), http.StatusGone, "expired")
	assertError(t, authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"expires_at": nil}), http.StatusGone, "expired")
	assertError(t, authorizedJSON(env, http.MethodDelete, path, created.OwnerToken, map[string]any{}), http.StatusGone, "expired")
}
