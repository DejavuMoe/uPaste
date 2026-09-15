package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DejavuMoe/uPaste/internal/share"
)

func TestTextSizeBoundaries(t *testing.T) {
	env := newAPITestEnv(t)
	create := func(content string) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(map[string]any{
			"payload_kind": "TEXT",
			"privacy_mode": "STANDARD",
			"text":         map[string]any{"format": "PLAIN", "content": content},
			"expires_at":   nil,
		})
		if err != nil {
			t.Fatal(err)
		}
		return env.request(http.MethodPost, "/api/v1/shares", body)
	}

	if response := create(strings.Repeat("x", share.MaxTextBytes)); response.Code != http.StatusCreated {
		t.Fatalf("exact maximum Text status = %d; body=%s", response.Code, response.Body.String())
	}
	response := create(strings.Repeat("x", share.MaxTextBytes+1))
	assertError(t, response, http.StatusRequestEntityTooLarge, "request_too_large")

	// Multi-byte UTF-8 is counted in bytes, not characters.
	multibyte := strings.Repeat("é", share.MaxTextBytes/2)
	if response := create(multibyte); response.Code != http.StatusCreated {
		t.Fatalf("exact maximum multibyte Text status = %d", response.Code)
	}
	if response := create(multibyte + "é"); response.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("over-limit multibyte Text status = %d", response.Code)
	}
}
