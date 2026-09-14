package capability

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
	shareIDBytes      = 16
	ShareIDLength     = 22
	ownerTokenBytes   = 32
	ownerTokenPrefix  = "up_o1_"
	ownerRandomLength = 43
	OwnerTokenLength  = len(ownerTokenPrefix) + ownerRandomLength
)

type ShareID struct{ value string }

func GenerateShareID() (ShareID, error) {
	value, err := randomBase64URL(shareIDBytes)
	if err != nil {
		return ShareID{}, fmt.Errorf("generate Share ID: %w", err)
	}
	return ShareID{value: value}, nil
}

func ParseShareID(value string) (ShareID, error) {
	if err := validateBase64URL(value, ShareIDLength, shareIDBytes); err != nil {
		return ShareID{}, fmt.Errorf("invalid Share ID: %w", err)
	}
	return ShareID{value: value}, nil
}

func (id ShareID) String() string { return id.value }

type OwnerToken struct{ value string }

type Verifier [sha256.Size]byte

func GenerateOwnerToken() (OwnerToken, error) {
	random, err := randomBase64URL(ownerTokenBytes)
	if err != nil {
		return OwnerToken{}, fmt.Errorf("generate owner token: %w", err)
	}
	return OwnerToken{value: ownerTokenPrefix + random}, nil
}

func ParseOwnerToken(value string) (OwnerToken, error) {
	if len(value) != OwnerTokenLength || len(value) < len(ownerTokenPrefix) || value[:len(ownerTokenPrefix)] != ownerTokenPrefix {
		return OwnerToken{}, errors.New("invalid owner token shape")
	}
	if err := validateBase64URL(value[len(ownerTokenPrefix):], ownerRandomLength, ownerTokenBytes); err != nil {
		return OwnerToken{}, fmt.Errorf("invalid owner token: %w", err)
	}
	return OwnerToken{value: value}, nil
}

const redactedOwnerToken = "[REDACTED owner capability]"

func (token OwnerToken) Reveal() string { return token.value }

func (OwnerToken) String() string { return redactedOwnerToken }

func (OwnerToken) GoString() string { return redactedOwnerToken }

func (OwnerToken) LogValue() slog.Value { return slog.StringValue(redactedOwnerToken) }

func (token OwnerToken) Verifier() Verifier { return sha256.Sum256([]byte(token.value)) }

func VerifyOwnerToken(candidate string, expected Verifier) bool {
	token, err := ParseOwnerToken(candidate)
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

func validateBase64URL(value string, encodedLength, decodedLength int) error {
	if len(value) != encodedLength {
		return fmt.Errorf("length is %d, want %d", len(value), encodedLength)
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return errors.New("not unpadded base64url")
	}
	if len(decoded) != decodedLength || base64.RawURLEncoding.EncodeToString(decoded) != value {
		return errors.New("noncanonical encoding")
	}
	return nil
}
