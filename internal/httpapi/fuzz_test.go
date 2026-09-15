package httpapi

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func FuzzDecodeBase64URL(f *testing.F) {
	for _, seed := range []string{"", "A", "AA", "AAA", "AAAA", "AAAA=", "AAA!", strings.Repeat("A", 200)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		decoded, err := decodeBase64URL(value, 1, 64)
		if err != nil {
			return
		}
		if len(decoded) < 1 || len(decoded) > 64 {
			t.Fatalf("decoded length %d outside declared bounds", len(decoded))
		}
		if base64.RawURLEncoding.EncodeToString(decoded) != value {
			t.Fatalf("accepted noncanonical base64url value %q", value)
		}
		if strings.ContainsAny(value, "=+/") {
			t.Fatalf("accepted padded or non-url base64url value %q", value)
		}
	})
}

func FuzzSanitizeFilename(f *testing.F) {
	for _, seed := range []string{
		"",
		"report.pdf",
		"../../evil.html",
		`C:\\fakepath\\evil.svg`,
		"line\nbreak",
		"unicode-报告.pdf",
		strings.Repeat("x", 256),
		"..",
		".hidden",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got, err := sanitizeFilename(value)
		if err != nil {
			return
		}
		if got == "" {
			t.Fatal("sanitized filename is empty")
		}
		if len(got) > 255 {
			t.Fatalf("sanitized filename length %d exceeds 255", len(got))
		}
		if !utf8.ValidString(got) {
			t.Fatal("sanitized filename is invalid UTF-8")
		}
		if strings.ContainsAny(got, `/\`) {
			t.Fatalf("sanitized filename kept a path separator: %q", got)
		}
		for _, character := range got {
			if unicode.IsControl(character) {
				t.Fatalf("sanitized filename kept control rune %U", character)
			}
		}
	})
}
