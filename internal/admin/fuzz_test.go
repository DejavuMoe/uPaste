package admin

import (
	"encoding/base64"
	"testing"
)

func FuzzParseAdminToken(f *testing.F) {
	valid := TokenPrefix + base64.RawURLEncoding.EncodeToString(make([]byte, tokenRandomSize))
	for _, seed := range []string{"", valid, valid + "A", "up_a2_" + valid[len(TokenPrefix):], TokenPrefix + "!!!!"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		token, err := ParseToken(value)
		if err != nil {
			return
		}
		if token.Reveal() != value {
			t.Fatalf("Reveal() = %q, want %q", token.Reveal(), value)
		}
		if len(value) != TokenLength || value[:len(TokenPrefix)] != TokenPrefix {
			t.Fatalf("accepted malformed admin token %q", value)
		}
		decoded, err := base64.RawURLEncoding.DecodeString(value[len(TokenPrefix):])
		if err != nil || len(decoded) != tokenRandomSize {
			t.Fatalf("accepted noncanonical admin token %q", value)
		}
		if !VerifyToken(value, token.Verifier()) {
			t.Fatal("valid admin token failed verification")
		}
	})
}
