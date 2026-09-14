package httpapi

import (
	"bytes"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DejavuMoe/uPaste/internal/capability"
	"github.com/DejavuMoe/uPaste/internal/share"
)

func TestPatchAuthenticatesBeforeBodyValidation(t *testing.T) {
	env := newAPITestEnv(t)
	created := env.create("PLAIN", "original", nil)
	wrong, err := capability.GenerateOwnerToken()
	if err != nil {
		t.Fatal(err)
	}
	invalidUTF8 := append([]byte(`{"text":{"format":"PLAIN","content":"`), append([]byte{0xff}, []byte(`"}}`)...)...)
	tests := []struct {
		name   string
		body   []byte
		status int
	}{
		{"malformed JSON", []byte(`{`), 400},
		{"unknown field", []byte(`{"unknown":true}`), 400},
		{"duplicate field", []byte(`{"expires_at":null,"expires_at":null}`), 400},
		{"invalid UTF-8", invalidUTF8, 400},
		{"wire limit", bytes.Repeat([]byte("x"), maxJSONBytes+1), 413},
		{"text limit", []byte(`{"text":{"format":"PLAIN","content":"` + strings.Repeat("x", share.MaxTextBytes+1) + `"}}`), 413},
		{"invalid expiration", []byte(`{"expires_at":"2020-01-01T00:00:00Z"}`), 400},
		{"null text", []byte(`{"text":null}`), 400},
	}
	for _, test := range tests {
		t.Run(test.name+" unauthorized", func(t *testing.T) {
			response := patchBytes(env, created.Share.ID, wrong.Reveal(), test.body)
			assertError(t, response, http.StatusUnauthorized, "unauthorized")
			assertUnchangedShare(t, env, created.Share.ID)
		})
		t.Run(test.name+" authorized", func(t *testing.T) {
			response := patchBytes(env, created.Share.ID, created.OwnerToken, test.body)
			code := "invalid_request"
			if test.status == http.StatusRequestEntityTooLarge {
				code = "request_too_large"
			}
			assertError(t, response, test.status, code)
			assertUnchangedShare(t, env, created.Share.ID)
		})
	}
}

func patchBytes(env *apiTestEnv, id, token string, body []byte) *httptest.ResponseRecorder {
	env.t.Helper()
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/"+id, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	bearer(request, token)
	return env.serve(request)
}

func assertUnchangedShare(t *testing.T, env *apiTestEnv, id string) {
	t.Helper()
	var format, content string
	var expiresAt sql.NullInt64
	if err := env.db.QueryRow(`
		SELECT p.format, p.content, s.expires_at
		FROM shares s JOIN standard_text_payloads p ON p.share_id = s.id
		WHERE s.id = ?
	`, id).Scan(&format, &content, &expiresAt); err != nil {
		t.Fatal(err)
	}
	if format != "PLAIN" || content != "original" || expiresAt.Valid {
		t.Fatal("invalid PATCH mutated Share")
	}
}
