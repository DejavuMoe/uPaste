package challenge

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestConfigValidationAndPublicShape(t *testing.T) {
	if err := (Config{Provider: ProviderCap, CapEndpoint: "https://cap.example.com", CapSiteKey: "site", CapSecretKey: "secret"}).Validate(); err != nil {
		t.Fatalf("valid cap config rejected: %v", err)
	}
	if err := (Config{Provider: ProviderCap, CapEndpoint: "http://cap.example.com", CapSiteKey: "site", CapSecretKey: "secret"}).Validate(); err == nil {
		t.Fatal("non-loopback HTTP cap endpoint accepted")
	}
	if err := (Config{Provider: ProviderCap, CapEndpoint: "https://cap.example.com/siteverify", CapSiteKey: "site", CapSecretKey: "secret"}).Validate(); err == nil {
		t.Fatal("cap endpoint with path accepted")
	}
	if err := (Config{Provider: ProviderCap, CapEndpoint: "https://cap.example.com", CapSiteKey: "site"}).Validate(); err == nil {
		t.Fatal("cap secret missing accepted")
	}
	if err := (Config{Provider: ProviderTurnstile, TurnstileSiteKey: "site", TurnstileSecretKey: "secret", TurnstileHostname: "paste.example.com"}).Validate(); err != nil {
		t.Fatalf("valid turnstile config rejected: %v", err)
	}
	if err := (Config{Provider: ProviderTurnstile, TurnstileSiteKey: "site", TurnstileSecretKey: "secret"}).Validate(); err == nil {
		t.Fatal("turnstile hostname missing accepted")
	}

	public := (Config{Provider: ProviderCap, CapEndpoint: "https://cap.example.com", CapSiteKey: "site", CapSecretKey: "secret"}).PublicConfig()
	encoded, _ := json.Marshal(public)
	if strings.Contains(string(encoded), "secret") {
		t.Fatalf("public cap config leaked a secret: %s", encoded)
	}
	if public.Provider != "cap" || public.SiteKey != "site" || public.Endpoint != "https://cap.example.com" {
		t.Fatalf("public cap config = %+v", public)
	}
}

func TestCapVerifierContract(t *testing.T) {
	var received map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/siteverify" || r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	defer server.Close()

	verifier, err := NewVerifier(Config{Provider: ProviderCap, CapEndpoint: server.URL, CapSiteKey: "site", CapSecretKey: "cap-secret"}, nil, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifier.Verify(context.Background(), "cap-token", "198.51.100.1"); err != nil {
		t.Fatalf("cap verify failed: %v", err)
	}
	if received["secret"] != "cap-secret" || received["response"] != "cap-token" {
		t.Fatalf("cap siteverify payload = %+v", received)
	}

	failure := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":false}`))
	}))
	defer failure.Close()
	failingVerifier, _ := NewVerifier(Config{Provider: ProviderCap, CapEndpoint: failure.URL, CapSiteKey: "site", CapSecretKey: "cap-secret"}, nil, 2*time.Second)
	if err := failingVerifier.Verify(context.Background(), "cap-token", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("cap failure error = %v", err)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "down", http.StatusBadGateway) }))
	defer down.Close()
	downVerifier, _ := NewVerifier(Config{Provider: ProviderCap, CapEndpoint: down.URL, CapSiteKey: "site", CapSecretKey: "cap-secret"}, nil, 2*time.Second)
	if err := downVerifier.Verify(context.Background(), "cap-token", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("cap downstream error = %v", err)
	}
}

func TestTurnstileVerifierContract(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		_ = r.ParseForm()
		form = r.PostForm
		_, _ = w.Write([]byte(`{"success":true,"hostname":"paste.example.com","action":"create_share"}`))
	}))
	defer server.Close()

	verifier, err := NewVerifier(Config{Provider: ProviderTurnstile, TurnstileSiteKey: "site", TurnstileSecretKey: "turnstile-secret", TurnstileHostname: "paste.example.com"}, nil, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	verifier.(*turnstileVerifier).siteverifyURL = server.URL
	if err := verifier.Verify(context.Background(), "turnstile-token", "198.51.100.9"); err != nil {
		t.Fatalf("turnstile verify failed: %v", err)
	}
	if form.Get("secret") != "turnstile-secret" || form.Get("response") != "turnstile-token" || form.Get("remoteip") != "198.51.100.9" {
		t.Fatalf("turnstile form = %v", form)
	}

	for name, body := range map[string]string{
		"success false":      `{"success":false}`,
		"wrong hostname":     `{"success":true,"hostname":"evil.example.com","action":"create_share"}`,
		"wrong action":       `{"success":true,"hostname":"paste.example.com","action":"other"}`,
		"malformed response": `not-json`,
	} {
		t.Run(name, func(t *testing.T) {
			local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) }))
			defer local.Close()
			localVerifier, _ := NewVerifier(Config{Provider: ProviderTurnstile, TurnstileSiteKey: "site", TurnstileSecretKey: "turnstile-secret", TurnstileHostname: "paste.example.com"}, nil, 2*time.Second)
			localVerifier.(*turnstileVerifier).siteverifyURL = local.URL
			err := localVerifier.Verify(context.Background(), "token", "")
			if !errors.Is(err, ErrInvalid) && !errors.Is(err, ErrUnavailable) {
				t.Fatalf("error = %v, want invalid or unavailable", err)
			}
		})
	}
}
