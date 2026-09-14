package domain

import (
	"fmt"
	"time"
)

type PayloadKind string

const (
	PayloadText PayloadKind = "TEXT"
	PayloadFile PayloadKind = "FILE"
)

func ParsePayloadKind(value string) (PayloadKind, error) {
	kind := PayloadKind(value)
	if kind != PayloadText && kind != PayloadFile {
		return "", fmt.Errorf("invalid payload kind %q", value)
	}
	return kind, nil
}

type PrivacyMode string

const (
	PrivacyStandard  PrivacyMode = "STANDARD"
	PrivacyEncrypted PrivacyMode = "ENCRYPTED"
)

func ParsePrivacyMode(value string) (PrivacyMode, error) {
	mode := PrivacyMode(value)
	if mode != PrivacyStandard && mode != PrivacyEncrypted {
		return "", fmt.Errorf("invalid privacy mode %q", value)
	}
	return mode, nil
}

func ValidatePayloadPrivacy(kind PayloadKind, mode PrivacyMode) error {
	if _, err := ParsePayloadKind(string(kind)); err != nil {
		return err
	}
	if _, err := ParsePrivacyMode(string(mode)); err != nil {
		return err
	}
	if kind == PayloadFile && mode == PrivacyEncrypted {
		return fmt.Errorf("privacy mode %s is not supported for payload kind %s", mode, kind)
	}
	return nil
}

type TextFormat string

const (
	TextPlain    TextFormat = "PLAIN"
	TextSource   TextFormat = "SOURCE"
	TextMarkdown TextFormat = "MARKDOWN"
)

func ParseTextFormat(value string) (TextFormat, error) {
	format := TextFormat(value)
	if format != TextPlain && format != TextSource && format != TextMarkdown {
		return "", fmt.Errorf("invalid text format %q", value)
	}
	return format, nil
}

func IsExpired(expiresAt *time.Time, now time.Time) bool {
	return expiresAt != nil && !now.Before(*expiresAt)
}
