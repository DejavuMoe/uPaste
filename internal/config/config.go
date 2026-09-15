package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/netip"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DejavuMoe/uPaste/internal/admin"
	"github.com/DejavuMoe/uPaste/internal/challenge"
)

const (
	defaultAddr             = "127.0.0.1:8080"
	defaultFileAddr         = "127.0.0.1:8081"
	defaultFileOrigin       = "http://127.0.0.1:8081"
	defaultDataDir          = "./data"
	defaultPublicDefaultTTL = 24 * time.Hour
	defaultPublicMaxTTL     = 168 * time.Hour
	defaultChallengeTimeout = 10 * time.Second
)

type Mode string

const (
	ModePrivate Mode = "private"
	ModePublic  Mode = "public"
)

type LookupEnv func(string) (string, bool)

type Config struct {
	Mode           Mode
	Addr           string
	FileAddr       string
	FileOrigin     string
	TrustedProxies []netip.Prefix
	DataDir        string

	PublicDefaultTTL time.Duration
	PublicMaxTTL     time.Duration

	Challenge        challenge.Config
	ChallengeTimeout time.Duration

	AdminEnabled      bool
	AdminVerifier     admin.Verifier
	AdminCookieSecure bool
}

func (config Config) ChallengeEnabled() bool { return config.Challenge.Provider != "" }
func (config Config) Public() bool           { return config.Mode == ModePublic }

func Parse(args []string, lookup LookupEnv) (Config, error) {
	addr := envOrDefault(lookup, "UPASTE_ADDR", defaultAddr)
	fileAddr := envOrDefault(lookup, "UPASTE_FILE_ADDR", defaultFileAddr)
	fileOrigin := envOrDefault(lookup, "UPASTE_FILE_ORIGIN", defaultFileOrigin)
	trustedProxyCIDRs := envOrDefault(lookup, "UPASTE_TRUSTED_PROXY_CIDRS", "")
	dataDir := envOrDefault(lookup, "UPASTE_DATA_DIR", defaultDataDir)

	flags := flag.NewFlagSet("upaste", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&addr, "addr", addr, "application HTTP listen address")
	flags.StringVar(&fileAddr, "file-addr", fileAddr, "file HTTP listen address")
	flags.StringVar(&fileOrigin, "file-origin", fileOrigin, "public file origin")
	flags.StringVar(&trustedProxyCIDRs, "trusted-proxy-cidrs", trustedProxyCIDRs, "comma-separated trusted proxy CIDRs")
	flags.StringVar(&dataDir, "data-dir", dataDir, "runtime data directory")
	if err := flags.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}
	if flags.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	modeValue := strings.ToLower(strings.TrimSpace(envOrDefault(lookup, "UPASTE_DEPLOYMENT_MODE", string(ModePrivate))))
	mode := Mode(modeValue)
	if mode != ModePrivate && mode != ModePublic {
		return Config{}, fmt.Errorf("invalid UPASTE_DEPLOYMENT_MODE %q", modeValue)
	}

	publicDefaultTTL, err := envDuration(lookup, "UPASTE_PUBLIC_DEFAULT_TTL", defaultPublicDefaultTTL)
	if err != nil {
		return Config{}, err
	}
	publicMaxTTL, err := envDuration(lookup, "UPASTE_PUBLIC_MAX_TTL", defaultPublicMaxTTL)
	if err != nil {
		return Config{}, err
	}
	if publicDefaultTTL <= 0 || publicMaxTTL <= 0 || publicDefaultTTL > publicMaxTTL {
		return Config{}, errors.New("UPASTE_PUBLIC_DEFAULT_TTL must be > 0 and <= UPASTE_PUBLIC_MAX_TTL")
	}

	challengeTimeout, err := envDuration(lookup, "UPASTE_CHALLENGE_TIMEOUT", defaultChallengeTimeout)
	if err != nil {
		return Config{}, err
	}
	if challengeTimeout <= 0 {
		return Config{}, errors.New("UPASTE_CHALLENGE_TIMEOUT must be positive")
	}

	challengeConfig, err := parseChallenge(lookup)
	if err != nil {
		return Config{}, err
	}
	if mode == ModePrivate && challengeConfig.Provider != "" {
		return Config{}, errors.New("UPASTE_CHALLENGE_PROVIDER is only supported in public deployment mode")
	}
	if mode == ModePublic && challengeConfig.Provider == "" {
		return Config{}, errors.New("public deployment mode requires UPASTE_CHALLENGE_PROVIDER=cap or turnstile")
	}
	if challengeConfig.Provider != "" {
		if err := challengeConfig.Validate(); err != nil {
			return Config{}, fmt.Errorf("invalid challenge configuration: %w", err)
		}
	}

	adminEnabled := false
	var adminVerifier admin.Verifier
	adminToken := strings.TrimSpace(envOrDefault(lookup, "UPASTE_ADMIN_TOKEN", ""))
	if adminToken != "" {
		token, err := admin.ParseToken(adminToken)
		if err != nil {
			return Config{}, fmt.Errorf("invalid UPASTE_ADMIN_TOKEN: %w", err)
		}
		adminEnabled = true
		adminVerifier = token.Verifier()
	}
	if mode == ModePublic && !adminEnabled {
		return Config{}, errors.New("public deployment mode requires UPASTE_ADMIN_TOKEN")
	}
	adminCookieSecure := false
	if raw, ok := lookupEnv(lookup, "UPASTE_ADMIN_COOKIE_SECURE"); ok && strings.TrimSpace(raw) != "" {
		parsed, err := strconv.ParseBool(strings.TrimSpace(raw))
		if err != nil {
			return Config{}, fmt.Errorf("invalid UPASTE_ADMIN_COOKIE_SECURE: %w", err)
		}
		if mode == ModePublic && !parsed {
			return Config{}, errors.New("public deployment mode requires UPASTE_ADMIN_COOKIE_SECURE=true")
		}
		adminCookieSecure = parsed
	} else if mode == ModePublic {
		adminCookieSecure = true
	}

	addr = strings.TrimSpace(addr)
	fileAddr = strings.TrimSpace(fileAddr)
	if err := validateAddress(addr); err != nil {
		return Config{}, fmt.Errorf("invalid application listen address %q", addr)
	}
	if err := validateAddress(fileAddr); err != nil {
		return Config{}, fmt.Errorf("invalid file listen address %q", fileAddr)
	}
	if addr == fileAddr {
		return Config{}, errors.New("application and file listen addresses must differ")
	}
	fileOrigin, err = normalizeOrigin(fileOrigin)
	if err != nil {
		return Config{}, fmt.Errorf("invalid file origin: %w", err)
	}

	trustedProxies, err := parseCIDRs(trustedProxyCIDRs)
	if err != nil {
		return Config{}, fmt.Errorf("invalid trusted proxy CIDRs: %w", err)
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return Config{}, fmt.Errorf("data directory must not be empty")
	}
	dataDir, err = filepath.Abs(filepath.Clean(dataDir))
	if err != nil {
		return Config{}, fmt.Errorf("resolve data directory: %w", err)
	}

	return Config{
		Mode:              mode,
		Addr:              addr,
		FileAddr:          fileAddr,
		FileOrigin:        fileOrigin,
		TrustedProxies:    trustedProxies,
		DataDir:           dataDir,
		PublicDefaultTTL:  publicDefaultTTL,
		PublicMaxTTL:      publicMaxTTL,
		Challenge:         challengeConfig,
		ChallengeTimeout:  challengeTimeout,
		AdminEnabled:      adminEnabled,
		AdminVerifier:     adminVerifier,
		AdminCookieSecure: adminCookieSecure,
	}, nil
}

func parseChallenge(lookup LookupEnv) (challenge.Config, error) {
	providerValue := strings.TrimSpace(envOrDefault(lookup, "UPASTE_CHALLENGE_PROVIDER", ""))
	if providerValue == "" {
		return challenge.Config{}, nil
	}
	provider, err := challenge.ParseProvider(providerValue)
	if err != nil {
		return challenge.Config{}, err
	}
	return challenge.Config{
		Provider:           provider,
		CapEndpoint:        strings.TrimSpace(envOrDefault(lookup, "UPASTE_CAP_ENDPOINT", "")),
		CapSiteKey:         strings.TrimSpace(envOrDefault(lookup, "UPASTE_CAP_SITE_KEY", "")),
		CapSecretKey:       strings.TrimSpace(envOrDefault(lookup, "UPASTE_CAP_SECRET_KEY", "")),
		TurnstileSiteKey:   strings.TrimSpace(envOrDefault(lookup, "UPASTE_TURNSTILE_SITE_KEY", "")),
		TurnstileSecretKey: strings.TrimSpace(envOrDefault(lookup, "UPASTE_TURNSTILE_SECRET_KEY", "")),
		TurnstileHostname:  strings.TrimSpace(envOrDefault(lookup, "UPASTE_TURNSTILE_HOSTNAME", "")),
	}, nil
}

func envDuration(lookup LookupEnv, name string, fallback time.Duration) (time.Duration, error) {
	if lookup == nil {
		return fallback, nil
	}
	value, ok := lookup(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(strings.TrimSpace(value))
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return parsed, nil
}

func validateAddress(value string) error {
	host, port, err := net.SplitHostPort(value)
	if err != nil || host == "" {
		return errors.New("address requires host and port")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return errors.New("invalid port")
	}
	return nil
}

func normalizeOrigin(value string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.IsAbs() == false || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("origin must be an absolute HTTP(S) URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") || parsed.RawPath != "" {
		return "", errors.New("origin must not contain credentials, path, query, or fragment")
	}
	return parsed.Scheme + "://" + parsed.Host, nil
}

func parseCIDRs(value string) ([]netip.Prefix, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	prefixes := make([]netip.Prefix, 0, len(parts))
	for _, part := range parts {
		prefix, err := netip.ParsePrefix(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}

func lookupEnv(lookup LookupEnv, name string) (string, bool) {
	if lookup == nil {
		return "", false
	}
	return lookup(name)
}

func envOrDefault(lookup LookupEnv, name, fallback string) string {
	if lookup == nil {
		return fallback
	}
	if value, ok := lookup(name); ok {
		return value
	}
	return fallback
}
