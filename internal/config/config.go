package config

import (
	"flag"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultAddr    = "127.0.0.1:8080"
	defaultDataDir = "./data"
)

type LookupEnv func(string) (string, bool)

type Config struct {
	Addr    string
	DataDir string
}

func Parse(args []string, lookup LookupEnv) (Config, error) {
	addr := envOrDefault(lookup, "UPASTE_ADDR", defaultAddr)
	dataDir := envOrDefault(lookup, "UPASTE_DATA_DIR", defaultDataDir)

	flags := flag.NewFlagSet("upaste", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&addr, "addr", addr, "HTTP listen address")
	flags.StringVar(&dataDir, "data-dir", dataDir, "runtime data directory")
	if err := flags.Parse(args); err != nil {
		return Config{}, fmt.Errorf("parse flags: %w", err)
	}
	if flags.NArg() != 0 {
		return Config{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	addr = strings.TrimSpace(addr)
	host, port, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return Config{}, fmt.Errorf("invalid listen address %q", addr)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("invalid listen address %q", addr)
	}

	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return Config{}, fmt.Errorf("data directory must not be empty")
	}
	dataDir, err = filepath.Abs(filepath.Clean(dataDir))
	if err != nil {
		return Config{}, fmt.Errorf("resolve data directory: %w", err)
	}

	return Config{Addr: addr, DataDir: dataDir}, nil
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
