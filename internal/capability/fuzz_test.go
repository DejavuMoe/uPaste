package capability

import (
	"encoding/base64"
	"strings"
	"testing"
)

func FuzzParseShareID(f *testing.F) {
	for _, seed := range []string{
		"",
		"AAAAAAAAAAAAAAAAAAAAAA",
		"AAAAAAAAAAAAAAAAAAAAAA=",
		"AAAAAAAAAAAAAAAAAAAAA!",
		strings.Repeat("A", 64),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		id, err := ParseShareID(value)
		if err != nil {
			return
		}
		if len(value) != ShareIDLength {
			t.Fatalf("accepted Share ID length %d, want %d", len(value), ShareIDLength)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			t.Fatalf("accepted non-base64url Share ID: %v", err)
		}
		if len(decoded) != shareIDBytes {
			t.Fatalf("accepted decoded length %d, want %d", len(decoded), shareIDBytes)
		}
		if base64.RawURLEncoding.EncodeToString(decoded) != value {
			t.Fatalf("accepted noncanonical Share ID %q", value)
		}
		if id.String() != value {
			t.Fatalf("String() = %q, want %q", id.String(), value)
		}
		if _, err := ParseShareID(id.String()); err != nil {
			t.Fatalf("valid Share ID failed round-trip: %v", err)
		}
	})
}

func FuzzOwnerTokenParsing(f *testing.F) {
	valid := ownerTokenPrefix + base64.RawURLEncoding.EncodeToString(make([]byte, ownerTokenBytes))
	for _, seed := range []string{
		"",
		valid,
		valid + "=",
		"up_o2_" + strings.Repeat("A", ownerRandomLength),
		ownerTokenPrefix + strings.Repeat("A", ownerRandomLength+1),
		ownerTokenPrefix + strings.Repeat("!", ownerRandomLength),
		strings.Repeat("A", OwnerTokenLength),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		token, err := ParseOwnerToken(value)
		if err != nil {
			return
		}
		if token.Reveal() != value {
			t.Fatalf("Reveal() = %q, want %q", token.Reveal(), value)
		}
		if len(value) != OwnerTokenLength {
			t.Fatalf("accepted owner token length %d, want %d", len(value), OwnerTokenLength)
		}
		if !strings.HasPrefix(value, ownerTokenPrefix) {
			t.Fatalf("accepted owner token without prefix: %q", value)
		}
		random := value[len(ownerTokenPrefix):]
		decoded, err := base64.RawURLEncoding.DecodeString(random)
		if err != nil || len(decoded) != ownerTokenBytes {
			t.Fatalf("accepted noncanonical owner random portion: %v", err)
		}
		if base64.RawURLEncoding.EncodeToString(decoded) != random {
			t.Fatalf("accepted noncanonical owner token %q", value)
		}
		verifier := token.Verifier()
		if !VerifyOwnerToken(value, verifier) {
			t.Fatal("valid owner token failed verification")
		}
		if VerifyOwnerToken(value+"A", verifier) {
			t.Fatal("extended owner token verified")
		}
	})
}
