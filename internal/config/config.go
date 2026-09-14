package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultAddr       = "127.0.0.1:8080"
	defaultFileAddr   = "127.0.0.1:8081"
	defaultFileOrigin = "http://127.0.0.1:8081"
	defaultDataDir    = "./data"
)

type LookupEnv func(string) (string, bool)

type Config struct {
	Addr       string
	FileAddr   string
	FileOrigin string
	DataDir    string
}

func Parse(args []string, lookup LookupEnv) (Config, error) {
	addr := envOrDefault(lookup, "UPASTE_ADDR", defaultAddr)
	fileAddr := envOrDefault(lookup, "UPASTE_FILE_ADDR", defaultFileAddr)
	fileOrigin := envOrDefault(lookup, "UPASTE_FILE_ORIGIN", defaultFileOrigin)
	dataDir := envOrDefault(lookup, "UPASTE_DATA_DIR", defaultDataDir)

	flags := flag.NewFlagSet("upaste", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&addr, "addr", addr, "application HTTP listen address")
	flags.StringVar(&fileAddr, "file-addr", fileAddr, "file HTTP listen address")
	flags.StringVar(&fileOrigin, "file-origin", fileOrigin, "public file origin")
	flags.StringVar(&dataDir, "data-dir", dataDir, "runtime data directory")
	if err := flags.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}
	if flags.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
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
	fileOrigin, err := normalizeOrigin(fileOrigin)
	if err != nil {
		return Config{}, fmt.Errorf("invalid file origin: %w", err)
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return Config{}, fmt.Errorf("data directory must not be empty")
	}
	dataDir, err = filepath.Abs(filepath.Clean(dataDir))
	if err != nil {
		return Config{}, fmt.Errorf("resolve data directory: %w", err)
	}

	return Config{Addr: addr, FileAddr: fileAddr, FileOrigin: fileOrigin, DataDir: dataDir}, nil
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

func envOrDefault(lookup LookupEnv, name, fallback string) string {
	if lookup == nil {
		return fallback
	}
	if value, ok := lookup(name); ok {
		return value
	}
	return fallback
}
