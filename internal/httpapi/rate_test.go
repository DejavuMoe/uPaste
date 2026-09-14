package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DejavuMoe/uPaste/internal/abuse"
)

type unreadBody struct{}

func (unreadBody) Read([]byte) (int, error) { panic("rate-limited body was read") }
func (unreadBody) Close() error             { return nil }

func TestAPIRateLimitsBeforeBodies(t *testing.T) {
	env := newAPITestEnv(t)
	now := *env.now
	control := abuse.New(abuse.Config{Now: func() time.Time { return now }})
	env.handler = NewWithAbuse(env.shares, "https://files.example.test", env.log, control)
	for range 5 {
		response := env.request(http.MethodPost, "/api/v1/shares", []byte(`{}`))
		if response.Code != http.StatusBadRequest {
			t.Fatal(response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", unreadBody{})
	request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	response := env.serve(request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "60" || !strings.Contains(response.Body.String(), `"rate_limited"`) {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("API headers missing")
	}
}

func TestMutationRateLimitPrecedesAuthorization(t *testing.T) {
	env := newAPITestEnv(t)
	now := *env.now
	control := abuse.New(abuse.Config{Now: func() time.Time { return now }})
	env.handler = NewWithAbuse(env.shares, "https://files.example.test", env.log, control)
	for range 10 {
		request := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/not-an-id", strings.NewReader(`{`))
		request.Header.Set("Content-Type", "application/json")
		if response := env.serve(request); response.Code == http.StatusTooManyRequests {
			t.Fatal("early limit")
		}
	}
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/shares/not-an-id", io.NopCloser(unreadBody{}))
	request.Header.Set("Content-Type", "application/json")
	if response := env.serve(request); response.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d", response.Code)
	}
}

func TestFileUploadConcurrencyRejectsBeforeBody(t *testing.T) {
	env := newAPITestEnv(t)
	control := abuse.New(abuse.Config{})
	for range abuse.UploadLimit {
		if !control.TryUpload() {
			t.Fatal("slot")
		}
	}
	defer func() {
		for range abuse.UploadLimit {
			control.ReleaseUpload()
		}
	}()
	env.handler = NewWithAbuse(env.shares, "https://files.example.test", env.log, control)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/shares", unreadBody{})
	request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	response := env.serve(request)
	if response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") != "1" {
		t.Fatalf("status/retry = %d/%s", response.Code, response.Header().Get("Retry-After"))
	}
}
