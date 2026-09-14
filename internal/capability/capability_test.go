package capability

import (
	"encoding/base64"
	"regexp"
	"testing"
)

var urlSafe = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func TestGenerateShareID(t *testing.T) {
	seen := make(map[string]struct{}, 1000)
	for range 1000 {
		id, err := GenerateShareID()
		if err != nil {
			t.Fatal("GenerateShareID returned an error")
		}
		value := id.String()
		if len(value) != ShareIDLength {
			t.Fatalf("Share ID length = %d, want %d", len(value), ShareIDLength)
		}
		if !urlSafe.MatchString(value) {
			t.Fatal("Share ID contains a non-URL-safe character")
		}
		decoded, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil || len(decoded) != shareIDBytes {
			t.Fatalf("decoded Share ID length = %d, error = %v", len(decoded), err)
		}
		if _, err := ParseShareID(value); err != nil {
			t.Fatal("generated Share ID did not parse")
		}
		if _, exists := seen[value]; exists {
			t.Fatal("duplicate Share ID generated")
		}
		seen[value] = struct{}{}
	}
}

func TestParseShareIDRejectsMalformedValues(t *testing.T) {
	valid := base64.RawURLEncoding.EncodeToString(make([]byte, shareIDBytes))
	tests := []string{
		"",
		valid + "=",
		valid[:len(valid)-1],
		base64.RawURLEncoding.EncodeToString(make([]byte, shareIDBytes-1)),
		base64.RawURLEncoding.EncodeToString(make([]byte, shareIDBytes+1)),
		valid[:len(valid)-1] + "+",
		valid[:len(valid)-1] + "/",
		valid[:len(valid)-1] + "B", // non-zero unused trailing bits: noncanonical
	}
	for _, value := range tests {
		if _, err := ParseShareID(value); err == nil {
			t.Errorf("malformed Share ID of length %d unexpectedly parsed", len(value))
		}
	}
}

func TestOwnerToken(t *testing.T) {
	token, err := GenerateOwnerToken()
	if err != nil {
		t.Fatal("GenerateOwnerToken returned an error")
	}
	value := token.String()
	if len(value) != OwnerTokenLength {
		t.Fatalf("owner token length = %d, want %d", len(value), OwnerTokenLength)
	}
	if value[:len(ownerTokenPrefix)] != ownerTokenPrefix {
		t.Fatal("owner token has the wrong prefix")
	}
	random := value[len(ownerTokenPrefix):]
	if len(random) != ownerRandomLength || !urlSafe.MatchString(random) {
		t.Fatal("owner token random portion has the wrong shape")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(random)
	if err != nil || len(decoded) != ownerTokenBytes {
		t.Fatalf("decoded owner random length = %d, error = %v", len(decoded), err)
	}
	parsed, err := ParseOwnerToken(value)
	if err != nil {
		t.Fatal("generated owner token did not parse")
	}
	if parsed.Verifier() != token.Verifier() {
		t.Fatal("verifier is not deterministic")
	}
	if !VerifyOwnerToken(value, token.Verifier()) {
		t.Fatal("valid owner token did not verify")
	}

	other, err := GenerateOwnerToken()
	if err != nil {
		t.Fatal("second GenerateOwnerToken returned an error")
	}
	if VerifyOwnerToken(other.String(), token.Verifier()) {
		t.Fatal("different owner token verified")
	}
}

func TestParseOwnerTokenRejectsMalformedValues(t *testing.T) {
	random := base64.RawURLEncoding.EncodeToString(make([]byte, ownerTokenBytes))
	valid := ownerTokenPrefix + random
	tests := []string{
		"",
		"up_o2_" + random,
		ownerTokenPrefix + random[:len(random)-1],
		valid + "A",
		valid + "=",
		ownerTokenPrefix + random[:len(random)-1] + "+",
		ownerTokenPrefix + random[:len(random)-1] + "/",
		ownerTokenPrefix + random[:len(random)-1] + "B",
	}
	for _, value := range tests {
		if _, err := ParseOwnerToken(value); err == nil {
			t.Errorf("malformed owner token of length %d unexpectedly parsed", len(value))
		}
	}
}
