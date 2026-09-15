package config

import (
	"strings"
	"testing"

	"github.com/DejavuMoe/uPaste/internal/admin"
	"github.com/DejavuMoe/uPaste/internal/challenge"
)

func envMap(values map[string]string) LookupEnv {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func TestPrivateModeDefaultsRemainCompatible(t *testing.T) {
	config, err := Parse(nil, envMap(nil))
	if err != nil {
		t.Fatal(err)
	}
	if config.Mode != ModePrivate || config.Public() || config.ChallengeEnabled() || config.AdminEnabled {
		t.Fatalf("private defaults changed: %+v", config)
	}
	if config.PublicDefaultTTL.Hours() != 24 || config.PublicMaxTTL.Hours() != 168 {
		t.Fatalf("retention defaults = %v/%v", config.PublicDefaultTTL, config.PublicMaxTTL)
	}
}

func TestPublicModeFailsClosedWithoutChallengeOrAdmin(t *testing.T) {
	base := map[string]string{"UPASTE_DEPLOYMENT_MODE": "public"}
	if _, err := Parse(nil, envMap(base)); err == nil || !strings.Contains(err.Error(), "CHALLENGE_PROVIDER") {
		t.Fatalf("missing challenge error = %v", err)
	}
	token, err := admin.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	withChallenge := map[string]string{
		"UPASTE_DEPLOYMENT_MODE":    "public",
		"UPASTE_CHALLENGE_PROVIDER": "cap",
		"UPASTE_CAP_ENDPOINT":       "https://cap.example.com",
		"UPASTE_CAP_SITE_KEY":       "site",
		"UPASTE_CAP_SECRET_KEY":     "secret",
	}
	if _, err := Parse(nil, envMap(withChallenge)); err == nil || !strings.Contains(err.Error(), "ADMIN_TOKEN") {
		t.Fatalf("missing admin error = %v", err)
	}
	withChallenge["UPASTE_ADMIN_TOKEN"] = token.Reveal()
	config, err := Parse(nil, envMap(withChallenge))
	if err != nil {
		t.Fatal(err)
	}
	if !config.Public() || !config.ChallengeEnabled() || !config.AdminEnabled {
		t.Fatalf("public config = %+v", config)
	}
	if config.Challenge.Provider != challenge.ProviderCap || config.Challenge.CapSecretKey != "secret" {
		t.Fatalf("challenge config = %+v", config.Challenge)
	}
	if config.AdminVerifier != token.Verifier() {
		t.Fatal("admin verifier mismatch")
	}
}

func TestRetentionValidation(t *testing.T) {
	for name, values := range map[string]map[string]string{
		"default zero":     {"UPASTE_PUBLIC_DEFAULT_TTL": "0s"},
		"max zero":         {"UPASTE_PUBLIC_MAX_TTL": "0s"},
		"default over max": {"UPASTE_PUBLIC_DEFAULT_TTL": "200h", "UPASTE_PUBLIC_MAX_TTL": "168h"},
		"malformed":        {"UPASTE_PUBLIC_DEFAULT_TTL": "tomorrow"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(nil, envMap(values)); err == nil {
				t.Fatal("invalid retention accepted")
			}
		})
	}
}

func TestPrivateModeRejectsChallengeProvider(t *testing.T) {
	values := map[string]string{
		"UPASTE_CHALLENGE_PROVIDER":   "turnstile",
		"UPASTE_TURNSTILE_SITE_KEY":   "site",
		"UPASTE_TURNSTILE_SECRET_KEY": "secret",
		"UPASTE_TURNSTILE_HOSTNAME":   "paste.example.com",
	}
	if _, err := Parse(nil, envMap(values)); err == nil {
		t.Fatal("private mode accepted a challenge provider")
	}
}

func TestAdminTokenValidation(t *testing.T) {
	if _, err := Parse(nil, envMap(map[string]string{"UPASTE_ADMIN_TOKEN": "weak-password"})); err == nil {
		t.Fatal("weak admin token accepted")
	}
}

func TestAdminCookieSecureInvariant(t *testing.T) {
	token, err := admin.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	tokenValue := token.Reveal()
	basePublic := map[string]string{
		"UPASTE_DEPLOYMENT_MODE":    "public",
		"UPASTE_CHALLENGE_PROVIDER": "cap",
		"UPASTE_CAP_ENDPOINT":       "https://cap.example.com",
		"UPASTE_CAP_SITE_KEY":       "site",
		"UPASTE_CAP_SECRET_KEY":     "secret",
		"UPASTE_ADMIN_TOKEN":        tokenValue,
	}

	config, err := Parse(nil, envMap(basePublic))
	if err != nil {
		t.Fatal(err)
	}
	if !config.AdminCookieSecure {
		t.Fatal("public mode did not force Secure admin cookie")
	}

	downgraded := map[string]string{}
	for key, value := range basePublic {
		downgraded[key] = value
	}
	downgraded["UPASTE_ADMIN_COOKIE_SECURE"] = "false"
	if _, err := Parse(nil, envMap(downgraded)); err == nil {
		t.Fatal("public mode accepted an insecure admin cookie downgrade")
	}

	private, err := Parse(nil, envMap(nil))
	if err != nil {
		t.Fatal(err)
	}
	if private.AdminCookieSecure {
		t.Fatal("private mode default should not force Secure cookie")
	}
	privateSecure, err := Parse(nil, envMap(map[string]string{"UPASTE_ADMIN_COOKIE_SECURE": "true"}))
	if err != nil {
		t.Fatal(err)
	}
	if !privateSecure.AdminCookieSecure {
		t.Fatal("private mode explicit Secure cookie was not honored")
	}
}
