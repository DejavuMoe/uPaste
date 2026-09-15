package admin

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestTokenShapeAndRedaction(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	value := token.Reveal()
	if len(value) != TokenLength || !strings.HasPrefix(value, TokenPrefix) {
		t.Fatalf("generated admin token has wrong shape: len=%d", len(value))
	}
	parsed, err := ParseToken(value)
	if err != nil || parsed.Verifier() != token.Verifier() {
		t.Fatalf("ParseToken round-trip failed: %v", err)
	}
	for _, malformed := range []string{"", "up_a1_", "up_a2_" + value[len(TokenPrefix):], value + "A", value[:len(value)-1] + "!", value[:len(value)-2] + "=="} {
		if _, err := ParseToken(malformed); err == nil {
			t.Fatalf("accepted malformed admin token %q", malformed)
		}
	}
	var output bytes.Buffer
	slog.New(slog.NewTextHandler(&output, nil)).Info("test", "token", token)
	for name, formatted := range map[string]string{
		"slog": output.String(),
		"fmt":  fmt.Sprint(token),
		"go":   fmt.Sprintf("%#v", token),
	} {
		if strings.Contains(formatted, value) || strings.Contains(formatted, TokenPrefix) {
			t.Fatalf("%s formatting leaked admin token", name)
		}
		if !strings.Contains(formatted, redacted) {
			t.Fatalf("%s formatting did not mark token redacted", name)
		}
	}
}

func TestManagerLoginSessionsAndCSRF(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	manager := NewManager(token.Verifier(), Options{SessionTTL: 8 * time.Hour, Now: func() time.Time { return now }})

	if _, _, err := manager.Login("wrong", "198.51.100.1"); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("wrong token error = %v", err)
	}
	raw, csrf, err := manager.Login(token.Reveal(), "198.51.100.1")
	if err != nil {
		t.Fatal(err)
	}
	session, ok := manager.Lookup(raw)
	if !ok || session.CSRFToken != csrf || session.ExpiresAt != now.Add(8*time.Hour) {
		t.Fatalf("session lookup = %+v/%v", session, ok)
	}
	if !manager.CSRFMatches(raw, csrf) || manager.CSRFMatches(raw, csrf+"x") {
		t.Fatal("CSRF comparison is incorrect")
	}
	manager.Logout(raw)
	if _, ok := manager.Lookup(raw); ok {
		t.Fatal("logout did not invalidate session")
	}

	// Expiry.
	raw, _, err = manager.Login(token.Reveal(), "198.51.100.2")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(8*time.Hour + time.Second)
	if _, ok := manager.Lookup(raw); ok {
		t.Fatal("expired session remained valid")
	}
}

func TestManagerLoginRateLimitAndRestartInvalidation(t *testing.T) {
	token, err := GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager(token.Verifier(), Options{})
	for i := 0; i < loginBurst; i++ {
		if _, _, err := manager.Login("wrong", "203.0.113.7"); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("attempt %d error = %v", i, err)
		}
	}
	if _, _, err := manager.Login(token.Reveal(), "203.0.113.7"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("rate limit error = %v", err)
	}

	raw, _, err := manager.Login(token.Reveal(), "203.0.113.8")
	if err != nil {
		t.Fatal(err)
	}
	restarted := NewManager(token.Verifier(), Options{})
	if _, ok := restarted.Lookup(raw); ok {
		t.Fatal("server restart did not invalidate in-memory sessions")
	}
}
