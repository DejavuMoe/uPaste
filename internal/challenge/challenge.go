// Package challenge verifies anonymous-creation challenges through a configured
// provider. Verification is always server-side and fails closed.
package challenge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Provider string

const (
	ProviderCap       Provider = "cap"
	ProviderTurnstile Provider = "turnstile"
)

var (
	// ErrInvalid means the user's challenge could not be verified and the user
	// may retry with a fresh challenge.
	ErrInvalid = errors.New("challenge verification failed")
	// ErrUnavailable means the provider is unreachable or returned a temporary
	// failure; creation fails closed.
	ErrUnavailable = errors.New("challenge provider unavailable")
)

// Config holds the single configured challenge provider. Secrets live only in
// server memory and are never exposed through the public config endpoint.
type Config struct {
	Provider Provider

	CapEndpoint  string
	CapSiteKey   string
	CapSecretKey string

	TurnstileSiteKey   string
	TurnstileSecretKey string
	TurnstileHostname  string
}

func ParseProvider(value string) (Provider, error) {
	switch Provider(strings.ToLower(strings.TrimSpace(value))) {
	case ProviderCap:
		return ProviderCap, nil
	case ProviderTurnstile:
		return ProviderTurnstile, nil
	default:
		return "", fmt.Errorf("unsupported challenge provider %q", value)
	}
}

func (config Config) Validate() error {
	switch config.Provider {
	case ProviderCap:
		if _, err := normalizeEndpoint(config.CapEndpoint); err != nil {
			return fmt.Errorf("cap endpoint: %w", err)
		}
		if strings.TrimSpace(config.CapSiteKey) == "" {
			return errors.New("cap site key is required")
		}
		if strings.TrimSpace(config.CapSecretKey) == "" {
			return errors.New("cap secret key is required")
		}
	case ProviderTurnstile:
		if strings.TrimSpace(config.TurnstileSiteKey) == "" {
			return errors.New("turnstile site key is required")
		}
		if strings.TrimSpace(config.TurnstileSecretKey) == "" {
			return errors.New("turnstile secret key is required")
		}
		if strings.TrimSpace(config.TurnstileHostname) == "" {
			return errors.New("turnstile hostname is required")
		}
	default:
		return fmt.Errorf("unsupported challenge provider %q", config.Provider)
	}
	return nil
}

// PublicConfig is the non-secret challenge shape exposed to the frontend.
type PublicConfig struct {
	Provider string `json:"provider"`
	SiteKey  string `json:"site_key,omitempty"`
	Endpoint string `json:"api_endpoint,omitempty"`
	Hostname string `json:"hostname,omitempty"`
}

func (config Config) PublicConfig() PublicConfig {
	switch config.Provider {
	case ProviderCap:
		return PublicConfig{Provider: string(ProviderCap), SiteKey: config.CapSiteKey, Endpoint: config.CapEndpoint}
	case ProviderTurnstile:
		return PublicConfig{Provider: string(ProviderTurnstile), SiteKey: config.TurnstileSiteKey, Hostname: config.TurnstileHostname}
	default:
		return PublicConfig{}
	}
}

// Verifier verifies one challenge response token.
type Verifier interface {
	Verify(ctx context.Context, token, remoteIP string) error
}

// NewVerifier builds the configured provider verifier.
func NewVerifier(config Config, client *http.Client, timeout time.Duration) (Verifier, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if client == nil {
		client = &http.Client{}
	}
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	client = &http.Client{
		Transport:     client.Transport,
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       timeout,
	}
	switch config.Provider {
	case ProviderCap:
		endpoint, err := normalizeEndpoint(config.CapEndpoint)
		if err != nil {
			return nil, err
		}
		return &capVerifier{endpoint: endpoint, secret: config.CapSecretKey, client: client}, nil
	case ProviderTurnstile:
		return &turnstileVerifier{secret: config.TurnstileSecretKey, hostname: config.TurnstileHostname, client: client, siteverifyURL: turnstileSiteverifyURL}, nil
	default:
		return nil, fmt.Errorf("unsupported challenge provider %q", config.Provider)
	}
}

func normalizeEndpoint(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || !parsed.IsAbs() || parsed.Host == "" {
		return "", errors.New("must be an absolute HTTP(S) origin")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("must use HTTP(S)")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" {
		return "", errors.New("must be an origin without credentials, path, query, or fragment")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return "", errors.New("non-loopback endpoints must use HTTPS")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func isLoopbackHost(host string) bool {
	host = strings.Trim(host, "[]")
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
