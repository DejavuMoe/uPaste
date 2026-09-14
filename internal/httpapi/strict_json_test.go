package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateRejectsStrictJSONViolations(t *testing.T) {
	env := newAPITestEnv(t)
	validText := `{"format":"PLAIN","content":"x"}`
	tests := map[string][]byte{
		"duplicate payload_kind": []byte(`{"payload_kind":"TEXT","payload_kind":"FILE","privacy_mode":"STANDARD","text":` + validText + `,"expires_at":null}`),
		"duplicate privacy_mode": []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","privacy_mode":"ENCRYPTED","text":` + validText + `,"expires_at":null}`),
		"duplicate text":         []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":` + validText + `,"text":` + validText + `,"expires_at":null}`),
		"duplicate expires_at":   []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":` + validText + `,"expires_at":null,"expires_at":null}`),
		"duplicate text format":  []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","format":"MARKDOWN","content":"x"},"expires_at":null}`),
		"duplicate text content": []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"x","content":"y"},"expires_at":null}`),
		"case variant":           []byte(`{"Payload_Kind":"TEXT","privacy_mode":"STANDARD","text":` + validText + `,"expires_at":null}`),
		"invalid UTF-8":          append([]byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"`), append([]byte{0xff}, []byte(`"},"expires_at":null}`)...)...),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			assertInvalidBody(t, env.request(http.MethodPost, "/api/v1/shares", body))
		})
	}
	var count int
	if err := env.db.QueryRow("SELECT count(*) FROM shares").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("invalid create JSON persisted a Share")
	}
}

func TestJSONMediaTypesAndEncodings(t *testing.T) {
	env := newAPITestEnv(t)
	body := []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"x"},"expires_at":null}`)
	for name, configure := range map[string]func(*http.Request){
		"charset": func(r *http.Request) { r.Header.Set("Content-Type", "application/json; charset=utf-8") },
		"identity": func(r *http.Request) {
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Content-Encoding", "identity")
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewReader(body))
			configure(request)
			if response := env.serve(request); response.Code != http.StatusCreated {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
	for _, encoding := range []string{"gzip", "br", "deflate", "identity, gzip"} {
		t.Run(encoding, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Content-Encoding", encoding)
			assertError(t, env.serve(request), http.StatusUnsupportedMediaType, "unsupported_media_type")
		})
	}
}

func TestValidUTF8IsDecodedWithoutNormalization(t *testing.T) {
	env := newAPITestEnv(t)
	body := []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"\u00e9 e\u0301 ☃"},"expires_at":null}`)
	response := env.request(http.MethodPost, "/api/v1/shares", body)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", response.Code, response.Body.String())
	}
	var created createResponse
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Share.Text.Content != "é e\u0301 ☃" {
		t.Fatalf("content was changed: %q", created.Share.Text.Content)
	}
}

func TestPatchRejectsStrictJSONViolationsWithoutMutation(t *testing.T) {
	env := newAPITestEnv(t)
	created := env.create("PLAIN", "original", nil)
	path := "/api/v1/shares/" + created.Share.ID
	tests := map[string][]byte{
		"duplicate text":         []byte(`{"text":{"format":"PLAIN","content":"x"},"text":{"format":"PLAIN","content":"y"}}`),
		"duplicate expires_at":   []byte(`{"expires_at":null,"expires_at":null}`),
		"duplicate text format":  []byte(`{"text":{"format":"PLAIN","format":"MARKDOWN","content":"x"}}`),
		"duplicate text content": []byte(`{"text":{"format":"PLAIN","content":"x","content":"y"}}`),
		"case variant":           []byte(`{"Expires_At":null}`),
		"invalid UTF-8":          append([]byte(`{"text":{"format":"PLAIN","content":"`), append([]byte{0xff}, []byte(`"}}`)...)...),
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			bearer(request, created.OwnerToken)
			assertInvalidBody(t, env.serve(request))
			var format, content string
			if err := env.db.QueryRow("SELECT format, content FROM standard_text_payloads WHERE share_id = ?", created.Share.ID).Scan(&format, &content); err != nil {
				t.Fatal(err)
			}
			if format != "PLAIN" || content != "original" {
				t.Fatal("invalid patch JSON mutated text")
			}
		})
	}
}

func assertInvalidBody(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	assertError(t, response, http.StatusBadRequest, "invalid_request")
	var decoded errorResponse
	if err := json.Unmarshal(response.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Error.Message != "request body is invalid" {
		t.Fatalf("error message = %q", decoded.Error.Message)
	}
}
