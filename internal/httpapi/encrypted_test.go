package httpapi

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/share"
)

func encryptedBody(nonce, ciphertext []byte, expiresAt any) map[string]any {
	return map[string]any{
		"payload_kind": "TEXT", "privacy_mode": "ENCRYPTED",
		"encrypted_text": map[string]any{
			"protocol":   share.EncryptedTextProtocolV1,
			"nonce":      base64.RawURLEncoding.EncodeToString(nonce),
			"ciphertext": base64.RawURLEncoding.EncodeToString(ciphertext),
		},
		"expires_at": expiresAt,
	}
}

func (env *apiTestEnv) createEncrypted(nonce, ciphertext []byte, expiresAt any) createResponse {
	env.t.Helper()
	body, err := json.Marshal(encryptedBody(nonce, ciphertext, expiresAt))
	if err != nil {
		env.t.Fatal(err)
	}
	response := env.request(http.MethodPost, "/api/v1/shares", body)
	if response.Code != http.StatusCreated {
		env.t.Fatalf("encrypted create status = %d; body=%s", response.Code, response.Body.String())
	}
	var created createResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		env.t.Fatal(err)
	}
	return created
}

func TestEncryptedCreateGetAndZeroKnowledgeStorage(t *testing.T) {
	env := newAPITestEnv(t)
	marker := "ZERO_KNOWLEDGE_SECRET_MARKER"
	key := make([]byte, 32)
	nonce := make([]byte, 12)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(nonce); err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext := gcm.Seal(nil, nonce, append([]byte{1, 1}, marker...), []byte("uPaste:encrypted-text:v1"))
	requestBody, err := json.Marshal(encryptedBody(nonce, ciphertext, nil))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(requestBody, []byte(marker)) {
		t.Fatal("encrypted API request contains plaintext")
	}
	response := env.request(http.MethodPost, "/api/v1/shares", requestBody)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	var created createResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Share.PrivacyMode != "ENCRYPTED" || created.Share.Text != nil || created.Share.EncryptedText == nil {
		t.Fatal("encrypted response payload shape is incorrect")
	}
	responseText := response.Body.String()
	for _, forbidden := range []string{marker, "\"text\"", "\"format\"", "decryption_key", "key_fragment", "owner_token_verifier"} {
		if strings.Contains(responseText, forbidden) {
			t.Fatalf("encrypted create response contains %q", forbidden)
		}
	}

	var protocol string
	var storedNonce, storedCiphertext []byte
	if err := env.db.QueryRow("SELECT protocol, nonce, ciphertext FROM encrypted_text_payloads WHERE share_id = ?", created.Share.ID).Scan(&protocol, &storedNonce, &storedCiphertext); err != nil {
		t.Fatal(err)
	}
	if protocol != share.EncryptedTextProtocolV1 || !bytes.Equal(storedNonce, nonce) || !bytes.Equal(storedCiphertext, ciphertext) {
		t.Fatal("encrypted payload changed in storage")
	}
	rows, err := env.db.Query("SELECT name FROM pragma_table_info('encrypted_text_payloads') ORDER BY cid")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		columns = append(columns, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(columns, ",") != "share_id,protocol,nonce,ciphertext" {
		t.Fatalf("encrypted storage columns = %v", columns)
	}
	for _, visible := range [][]byte{requestBody, response.Body.Bytes(), storedNonce, storedCiphertext, env.logs.Bytes()} {
		if bytes.Contains(visible, []byte(marker)) || bytes.Contains(visible, key) {
			t.Fatal("server-visible material exposed plaintext or key")
		}
	}

	read := env.request(http.MethodGet, "/api/v1/shares/"+created.Share.ID, nil)
	if read.Code != http.StatusOK || strings.Contains(read.Body.String(), marker) || strings.Contains(read.Body.String(), "\"text\"") {
		t.Fatalf("encrypted GET response is invalid: status=%d", read.Code)
	}
	var got shareResponse
	if err := json.Unmarshal(read.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Share.EncryptedText == nil || got.Share.EncryptedText.Ciphertext != base64.RawURLEncoding.EncodeToString(ciphertext) {
		t.Fatal("encrypted GET changed ciphertext")
	}
}

func TestEncryptedCreateValidationAndLimits(t *testing.T) {
	env := newAPITestEnv(t)
	clone := func() map[string]any {
		body := encryptedBody(make([]byte, 12), make([]byte, 19), nil)
		return body
	}
	tests := []struct {
		name   string
		body   map[string]any
		status int
	}{
		{"both payloads", func() map[string]any {
			v := clone()
			v["text"] = map[string]any{"format": "PLAIN", "content": "x"}
			return v
		}(), 400},
		{"null text with encrypted payload", func() map[string]any {
			v := clone()
			v["text"] = nil
			return v
		}(), 400},
		{"null encrypted field with Standard payload", map[string]any{
			"payload_kind": "TEXT", "privacy_mode": "STANDARD",
			"text": map[string]any{"format": "PLAIN", "content": "x"}, "encrypted_text": nil, "expires_at": nil,
		}, 400},
		{"missing encrypted payload", map[string]any{"payload_kind": "TEXT", "privacy_mode": "ENCRYPTED", "expires_at": nil}, 400},
		{"unknown protocol", func() map[string]any {
			v := clone()
			v["encrypted_text"].(map[string]any)["protocol"] = "OTHER"
			return v
		}(), 400},
		{"malformed nonce", func() map[string]any {
			v := clone()
			v["encrypted_text"].(map[string]any)["nonce"] = "++++++++++++++++"
			return v
		}(), 400},
		{"nonce padding", func() map[string]any {
			v := clone()
			v["encrypted_text"].(map[string]any)["nonce"] = "AAAAAAAAAAAAAAAA="
			return v
		}(), 400},
		{"nonce length", func() map[string]any {
			v := clone()
			v["encrypted_text"].(map[string]any)["nonce"] = "AAAAAAAAAAAAAAA"
			return v
		}(), 400},
		{"malformed ciphertext", func() map[string]any {
			v := clone()
			v["encrypted_text"].(map[string]any)["ciphertext"] = "+"
			return v
		}(), 400},
		{"noncanonical ciphertext", func() map[string]any {
			v := clone()
			v["encrypted_text"].(map[string]any)["ciphertext"] = strings.Repeat("A", 25) + "B"
			return v
		}(), 400},
		{"small ciphertext", func() map[string]any {
			v := clone()
			v["encrypted_text"].(map[string]any)["ciphertext"] = base64.RawURLEncoding.EncodeToString(make([]byte, 18))
			return v
		}(), 400},
		{"oversized ciphertext", encryptedBody(make([]byte, 12), make([]byte, share.MaxEncryptedCipherBytes+1), nil), 413},
		{"key rejected", func() map[string]any {
			v := clone()
			v["encrypted_text"].(map[string]any)["key"] = "not-accepted"
			return v
		}(), 400},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := authorizedJSON(env, http.MethodPost, "/api/v1/shares", "", test.body)
			code := "invalid_request"
			if test.status == 413 {
				code = "request_too_large"
			}
			assertError(t, response, test.status, code)
		})
	}

	minimum := env.createEncrypted(make([]byte, 12), make([]byte, share.MinEncryptedCipherBytes), nil)
	if minimum.Share.EncryptedText == nil {
		t.Fatal("minimum encrypted payload was not returned")
	}
	maximumBody, err := json.Marshal(encryptedBody(make([]byte, 12), make([]byte, share.MaxEncryptedCipherBytes), nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(maximumBody) >= maxJSONBytes {
		t.Fatalf("maximum encrypted request size = %d, exceeds wire limit", len(maximumBody))
	}
	if response := env.request(http.MethodPost, "/api/v1/shares", maximumBody); response.Code != http.StatusCreated {
		t.Fatalf("maximum encrypted payload status = %d; wire bytes=%d", response.Code, len(maximumBody))
	}
}

func TestEncryptedPayloadRejectsDuplicateMembers(t *testing.T) {
	env := newAPITestEnv(t)
	nonce := base64.RawURLEncoding.EncodeToString(make([]byte, 12))
	ciphertext := base64.RawURLEncoding.EncodeToString(make([]byte, 19))
	for _, duplicate := range []string{
		`"protocol":"UPASTE_AES_GCM_V1"`,
		`"nonce":"` + nonce + `"`,
		`"ciphertext":"` + ciphertext + `"`,
	} {
		body := []byte(`{"payload_kind":"TEXT","privacy_mode":"ENCRYPTED","encrypted_text":{"protocol":"UPASTE_AES_GCM_V1","nonce":"` + nonce + `","ciphertext":"` + ciphertext + `",` + duplicate + `},"expires_at":null}`)
		assertInvalidBody(t, env.request(http.MethodPost, "/api/v1/shares", body))
	}
	created := env.createEncrypted(make([]byte, 12), make([]byte, 19), nil)
	for _, duplicate := range []string{
		`"protocol":"UPASTE_AES_GCM_V1"`,
		`"nonce":"` + nonce + `"`,
		`"ciphertext":"` + ciphertext + `"`,
	} {
		body := []byte(`{"encrypted_text":{"protocol":"UPASTE_AES_GCM_V1","nonce":"` + nonce + `","ciphertext":"` + ciphertext + `",` + duplicate + `}}`)
		assertInvalidBody(t, patchBytes(env, created.Share.ID, created.OwnerToken, body))
	}
}

func TestEncryptedRawExpirationAndDelete(t *testing.T) {
	env := newAPITestEnv(t)
	expires := env.now.Add(time.Hour)
	created := env.createEncrypted(make([]byte, 12), make([]byte, 19), expires.Format(time.RFC3339Nano))
	raw := env.request(http.MethodGet, "/raw/"+created.Share.ID, nil)
	if raw.Code != http.StatusConflict || strings.TrimSpace(raw.Body.String()) != "raw view unavailable for encrypted share" {
		t.Fatalf("encrypted raw status/body = %d/%q", raw.Code, raw.Body.String())
	}
	assertRawHeaders(t, raw)
	*env.now = expires
	if response := env.request(http.MethodGet, "/api/v1/shares/"+created.Share.ID, nil); response.Code != http.StatusGone {
		t.Fatalf("expired encrypted GET status = %d", response.Code)
	}
	expiredRaw := env.request(http.MethodGet, "/raw/"+created.Share.ID, nil)
	if expiredRaw.Code != http.StatusGone {
		t.Fatalf("expired encrypted raw status = %d", expiredRaw.Code)
	}
	assertRawHeaders(t, expiredRaw)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/shares/"+created.Share.ID, nil)
	bearer(request, created.OwnerToken)
	assertError(t, env.serve(request), http.StatusGone, "expired")

	*env.now = expires.Add(-time.Minute)
	active := env.createEncrypted(make([]byte, 12), make([]byte, 19), nil)
	path := "/api/v1/shares/" + active.Share.ID
	request = httptest.NewRequest(http.MethodDelete, path, nil)
	bearer(request, active.OwnerToken)
	if response := env.serve(request); response.Code != http.StatusNoContent {
		t.Fatalf("encrypted DELETE status = %d", response.Code)
	}
	var count int
	if err := env.db.QueryRow("SELECT count(*) FROM encrypted_text_payloads WHERE share_id = ?", active.Share.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("encrypted cascade count/error = %d/%v", count, err)
	}
	request = httptest.NewRequest(http.MethodDelete, path, nil)
	bearer(request, active.OwnerToken)
	assertError(t, env.serve(request), http.StatusNotFound, "not_found")
}
