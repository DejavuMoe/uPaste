// Package admin implements the single Superadmin authority: one high-entropy
// operator token, short-lived in-memory sessions, and CSRF-bound governance.
package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
)

const (
	TokenPrefix     = "up_a1_"
	tokenRandomSize = 32
	tokenRandomLen  = 43
	TokenLength     = len(TokenPrefix) + tokenRandomLen
)

// Verifier is the SHA-256 verifier of a canonical admin token.
type Verifier [sha256.Size]byte

// Token is a validated canonical admin token. It never formats its secret.
type Token struct{ value string }

func GenerateToken() (Token, error) {
	random, err := randomBase64URL(tokenRandomSize)
	if err != nil {
		return Token{}, fmt.Errorf("generate admin token: %w", err)
	}
	return Token{value: TokenPrefix + random}, nil
}

func ParseToken(value string) (Token, error) {
	if len(value) != TokenLength || len(value) < len(TokenPrefix) || value[:len(TokenPrefix)] != TokenPrefix {
		return Token{}, errors.New("invalid admin token shape")
	}
	if err := validateCanonical(value[len(TokenPrefix):], tokenRandomSize); err != nil {
		return Token{}, err
	}
	return Token{value: value}, nil
}

func (token Token) Reveal() string { return token.value }

func (token Token) Verifier() Verifier { return sha256.Sum256([]byte(token.value)) }

const redacted = "[REDACTED admin token]"

func (Token) String() string { return redacted }

func (Token) GoString() string { return redacted }

func (Token) LogValue() slog.Value { return slog.StringValue(redacted) }

// VerifyToken compares a candidate against a stored verifier in constant time.
func VerifyToken(candidate string, expected Verifier) bool {
	token, err := ParseToken(candidate)
	if err != nil {
		return false
	}
	actual := token.Verifier()
	return subtle.ConstantTimeCompare(actual[:], expected[:]) == 1
}

func randomBase64URL(size int) (string, error) {
	value := make([]byte, size)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func validateCanonical(value string, decodedSize int) error {
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return errors.New("admin token is not unpadded base64url")
	}
	if len(decoded) != decodedSize || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return errors.New("admin token is not canonical")
	}
	return nil
}
