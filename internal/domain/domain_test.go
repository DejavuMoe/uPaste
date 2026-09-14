package domain

import (
	"testing"
	"time"
)

func TestParsers(t *testing.T) {
	tests := []struct {
		name  string
		valid []string
		parse func(string) error
	}{
		{"payload kind", []string{"TEXT", "FILE"}, func(value string) error { _, err := ParsePayloadKind(value); return err }},
		{"privacy mode", []string{"STANDARD", "ENCRYPTED"}, func(value string) error { _, err := ParsePrivacyMode(value); return err }},
		{"text format", []string{"PLAIN", "SOURCE", "MARKDOWN"}, func(value string) error { _, err := ParseTextFormat(value); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range test.valid {
				if err := test.parse(value); err != nil {
					t.Errorf("parse %q: %v", value, err)
				}
			}
			for _, value := range []string{"", "text", "UNKNOWN"} {
				if err := test.parse(value); err == nil {
					t.Errorf("parse %q unexpectedly succeeded", value)
				}
			}
		})
	}
}

func TestValidatePayloadPrivacy(t *testing.T) {
	tests := []struct {
		kind PayloadKind
		mode PrivacyMode
		want bool
	}{
		{PayloadText, PrivacyStandard, true},
		{PayloadText, PrivacyEncrypted, true},
		{PayloadFile, PrivacyStandard, true},
		{PayloadFile, PrivacyEncrypted, false},
		{PayloadKind("OTHER"), PrivacyStandard, false},
		{PayloadText, PrivacyMode("OTHER"), false},
	}
	for _, test := range tests {
		if got := ValidatePayloadPrivacy(test.kind, test.mode) == nil; got != test.want {
			t.Errorf("ValidatePayloadPrivacy(%q, %q) valid = %v, want %v", test.kind, test.mode, got, test.want)
		}
	}
}

func TestIsExpired(t *testing.T) {
	expiresAt := time.Date(2026, time.September, 14, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		expiresAt *time.Time
		now       time.Time
		want      bool
	}{
		{"no expiration", nil, expiresAt.Add(time.Hour), false},
		{"before", &expiresAt, expiresAt.Add(-time.Nanosecond), false},
		{"at boundary", &expiresAt, expiresAt, true},
		{"after", &expiresAt, expiresAt.Add(time.Nanosecond), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsExpired(test.expiresAt, test.now); got != test.want {
				t.Errorf("IsExpired() = %v, want %v", got, test.want)
			}
		})
	}
}
