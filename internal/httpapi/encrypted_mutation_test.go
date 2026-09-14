package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/share"
)

func encryptedPatch(nonce, ciphertext []byte) map[string]any {
	return map[string]any{"encrypted_text": map[string]any{
		"protocol":   share.EncryptedTextProtocolV1,
		"nonce":      base64.RawURLEncoding.EncodeToString(nonce),
		"ciphertext": base64.RawURLEncoding.EncodeToString(ciphertext),
	}}
}

func TestEncryptedPatch(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(2 * time.Hour)
	created := env.createEncrypted(make([]byte, 12), make([]byte, 19), expires.Format(time.RFC3339Nano))
	path := "/api/v1/shares/" + created.Share.ID
	*env.now = env.now.Add(time.Hour)

	nonce := bytesFilled(12, 1)
	ciphertext := bytesFilled(20, 2)
	response := authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, encryptedPatch(nonce, ciphertext))
	if response.Code != http.StatusOK {
		t.Fatalf("encrypted PATCH status = %d; body=%s", response.Code, response.Body.String())
	}
	var patched shareResponse
	if err := json.Unmarshal(response.Body.Bytes(), &patched); err != nil {
		t.Fatal(err)
	}
	if patched.Share.Text != nil || patched.Share.EncryptedText == nil || strings.Contains(response.Body.String(), "owner_token") || strings.Contains(response.Body.String(), "key_fragment") {
		t.Fatal("encrypted PATCH exposed owner, key, or plaintext format material")
	}
	var storedNonce, storedCiphertext []byte
	var storedExpiration int64
	if err := env.db.QueryRow(`
		SELECT p.nonce, p.ciphertext, s.expires_at
		FROM encrypted_text_payloads p JOIN shares s ON s.id = p.share_id
		WHERE p.share_id = ?
	`, created.Share.ID).Scan(&storedNonce, &storedCiphertext, &storedExpiration); err != nil {
		t.Fatal(err)
	}
	if string(storedNonce) != string(nonce) || string(storedCiphertext) != string(ciphertext) || storedExpiration != expires.UnixMilli() {
		t.Fatal("encrypted replacement or omitted expiration was not preserved")
	}

	future := env.now.Add(3 * time.Hour)
	response = authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"expires_at": future.Format(time.RFC3339Nano)})
	if response.Code != http.StatusOK {
		t.Fatalf("encrypted expiration PATCH status = %d", response.Code)
	}
	response = authorizedJSON(env, http.MethodPatch, path, created.OwnerToken, map[string]any{"expires_at": nil})
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"expires_at":null`) {
		t.Fatalf("encrypted clear expiration status/body = %d/%s", response.Code, response.Body.String())
	}
}

func TestEncryptedPatchValidationAndAuthorization(t *testing.T) {
	env := newAPITestEnv(t)
	encrypted := env.createEncrypted(make([]byte, 12), make([]byte, 19), nil)
	encryptedPath := "/api/v1/shares/" + encrypted.Share.ID
	standard := env.create("PLAIN", "standard", nil)
	standardPath := "/api/v1/shares/" + standard.Share.ID

	assertError(t, authorizedJSON(env, http.MethodPatch, standardPath, standard.OwnerToken, encryptedPatch(make([]byte, 12), make([]byte, 19))), 400, "invalid_request")
	assertError(t, authorizedJSON(env, http.MethodPatch, encryptedPath, encrypted.OwnerToken, map[string]any{"text": map[string]any{"format": "PLAIN", "content": "x"}}), 400, "invalid_request")
	assertError(t, authorizedJSON(env, http.MethodPatch, encryptedPath, encrypted.OwnerToken, map[string]any{"privacy_mode": "STANDARD"}), 400, "invalid_request")
	assertError(t, authorizedJSON(env, http.MethodPatch, encryptedPath, encrypted.OwnerToken, map[string]any{"encrypted_text": map[string]any{"protocol": share.EncryptedTextProtocolV1, "nonce": "bad", "ciphertext": "bad"}}), 400, "invalid_request")
	assertError(t, authorizedJSON(env, http.MethodPatch, encryptedPath, encrypted.OwnerToken, map[string]any{"encrypted_text": map[string]any{"protocol": share.EncryptedTextProtocolV1, "nonce": base64.RawURLEncoding.EncodeToString(make([]byte, 12)), "ciphertext": base64.RawURLEncoding.EncodeToString(make([]byte, share.MaxEncryptedCipherBytes+1))}}), 413, "request_too_large")
	assertError(t, authorizedJSON(env, http.MethodPatch, encryptedPath, encrypted.OwnerToken, map[string]any{"encrypted_text": map[string]any{"protocol": share.EncryptedTextProtocolV1, "nonce": base64.RawURLEncoding.EncodeToString(make([]byte, 12)), "ciphertext": base64.RawURLEncoding.EncodeToString(make([]byte, 19)), "key": "rejected"}}), 400, "invalid_request")

	wrong, err := capability.GenerateOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPatch, encryptedPath, strings.NewReader(`{"encrypted_text":{`))
	request.Header.Set("Content-Type", "application/json")
	bearer(request, wrong.Reveal())
	assertError(t, env.serve(request), 401, "unauthorized")
	request = httptest.NewRequest(http.MethodPatch, encryptedPath, strings.NewReader(`{"encrypted_text":{`))
	request.Header.Set("Content-Type", "application/json")
	bearer(request, encrypted.OwnerToken)
	assertError(t, env.serve(request), 400, "invalid_request")
	request = httptest.NewRequest(http.MethodPatch, encryptedPath, strings.NewReader(`{"expires_at":null}`))
	request.Header.Set("Content-Type", "application/json")
	bearer(request, "up_e1_"+strings.Repeat("A", 43))
	assertError(t, env.serve(request), 401, "unauthorized")
}

func TestExpiredEncryptedShareCannotBeRevived(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(time.Hour)
	created := env.createEncrypted(make([]byte, 12), make([]byte, 19), expires.Format(time.RFC3339Nano))
	*env.now = expires
	body := encryptedPatch(bytesFilled(12, 3), bytesFilled(19, 4))
	body["expires_at"] = expires.Add(time.Hour).Format(time.RFC3339Nano)
	assertError(t, authorizedJSON(env, http.MethodPatch, "/api/v1/shares/"+created.Share.ID, created.OwnerToken, body), 410, "expired")
}

func bytesFilled(size int, value byte) []byte {
	result := make([]byte, size)
	for index := range result {
		result[index] = value
	}
	return result
}
