package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/abuse"
	"github.com/DejavuMoe/uPaste/internal/challenge"
)

type stubChallenge struct {
	err   error
	token string
}

func (stub *stubChallenge) Verify(_ context.Context, token, _ string) error {
	stub.token = token
	return stub.err
}

func newPublicEnv(t *testing.T, verifier challenge.Verifier) *apiTestEnv {
	t.Helper()
	env := newAPITestEnv(t)
	env.handler = NewWithOptions(env.shares, "https://files.example.test", env.log, disabledAbuseControl(), Options{
		Public:           true,
		PublicDefaultTTL: 24 * time.Hour,
		PublicMaxTTL:     168 * time.Hour,
		Challenge:        verifier,
		ChallengeConfig:  challenge.PublicConfig{Provider: "cap", SiteKey: "public-site-key", Endpoint: "https://cap.example.com"},
		ChallengeTimeout: time.Second,
		AdminEnabled:     true,
	})
	return env
}

func disabledAbuseControl() *abuse.Control {
	return abuse.New(abuse.Config{Disabled: true})
}

func TestPublicConfigEndpoint(t *testing.T) {
	private := newAPITestEnv(t)
	privateResponse := private.request(http.MethodGet, "/api/v1/config", nil)
	if privateResponse.Code != http.StatusOK {
		t.Fatalf("private config status = %d", privateResponse.Code)
	}
	var privateBody map[string]any
	if err := json.Unmarshal(privateResponse.Body.Bytes(), &privateBody); err != nil {
		t.Fatal(err)
	}
	if privateBody["deployment_mode"] != "private" || privateBody["admin_enabled"] != false {
		t.Fatalf("private config body = %v", privateBody)
	}
	if _, exists := privateBody["challenge"]; exists {
		t.Fatal("private config exposed a challenge")
	}

	env := newPublicEnv(t, &stubChallenge{})
	response := env.request(http.MethodGet, "/api/v1/config", nil)
	if response.Code != http.StatusOK {
		t.Fatalf("public config status = %d", response.Code)
	}
	raw := response.Body.String()
	for _, secret := range []string{"cap-secret", "admin-secret", "owner_token"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("public config leaked %q", secret)
		}
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	retention := body["retention"].(map[string]any)
	if retention["default_seconds"].(float64) != 86400 || retention["max_seconds"].(float64) != 604800 {
		t.Fatalf("retention = %v", retention)
	}
	challengeConfig := body["challenge"].(map[string]any)
	if challengeConfig["provider"] != "cap" || challengeConfig["site_key"] != "public-site-key" {
		t.Fatalf("challenge config = %v", challengeConfig)
	}
}

func TestChallengeRequiredForJSONAndMultipartCreation(t *testing.T) {
	verifier := &stubChallenge{}
	env := newPublicEnv(t, verifier)
	jsonBody := []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"x"},"expires_at":null}`)
	assertError(t, env.request(http.MethodPost, "/api/v1/shares", jsonBody), http.StatusForbidden, "challenge_required")
	response := env.serve(multipartRequest(t, `{"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null}`, "x.bin", []byte("x"), false))
	assertError(t, response, http.StatusForbidden, "challenge_required")

	// Read surfaces do not require a challenge.
	createdResponse := env.requestWithHeader(http.MethodPost, "/api/v1/shares", jsonBody, ChallengeHeader, "valid-token")
	if createdResponse.Code != http.StatusCreated {
		t.Fatalf("verified create status = %d; body=%s", createdResponse.Code, createdResponse.Body.String())
	}
	var created createResponse
	if err := json.Unmarshal(createdResponse.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if response := env.request(http.MethodGet, "/api/v1/shares/"+created.Share.ID, nil); response.Code != http.StatusOK {
		t.Fatalf("read without challenge status = %d", response.Code)
	}
}

func TestChallengeFailureClassificationAndTransport(t *testing.T) {
	verifier := &stubChallenge{}
	env := newPublicEnv(t, verifier)

	request := env.requestWithHeader(http.MethodPost, "/api/v1/shares", []byte(`{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"ok"},"expires_at":null}`), ChallengeHeader, "challenge-token")
	if request.Code != http.StatusCreated {
		t.Fatalf("successful challenge create status = %d; body=%s", request.Code, request.Body.String())
	}
	if verifier.token != "challenge-token" {
		t.Fatalf("verifier token = %q", verifier.token)
	}
	if strings.Contains(env.logs.String(), "challenge-token") {
		t.Fatal("challenge token was logged")
	}

	verifier.err = challenge.ErrInvalid
	assertError(t, env.requestWithHeader(http.MethodPost, "/api/v1/shares", []byte(`{}`), ChallengeHeader, "bad"), http.StatusForbidden, "challenge_failed")

	verifier.err = challenge.ErrUnavailable
	assertError(t, env.requestWithHeader(http.MethodPost, "/api/v1/shares", []byte(`{}`), ChallengeHeader, "bad"), http.StatusServiceUnavailable, "challenge_unavailable")

	verifier.err = nil
	request = env.requestWithHeader(http.MethodPost, "/api/v1/shares", []byte(`{}`), ChallengeHeader, " ")
	assertError(t, request, http.StatusForbidden, "challenge_required")
}

func (env *apiTestEnv) requestWithHeader(method, path string, body []byte, name, value string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(name, value)
	recorder := httptest.NewRecorder()
	env.handler.ServeHTTP(recorder, request)
	return recorder
}
