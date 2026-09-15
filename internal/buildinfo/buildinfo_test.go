package buildinfo

import (
	"runtime"
	"strings"
	"testing"
)

func TestDevelopmentDefaults(t *testing.T) {
	if Version != "devel" {
		t.Fatalf("Version = %q, want devel", Version)
	}
	if Commit != "unknown" {
		t.Fatalf("Commit = %q, want unknown", Commit)
	}
	if BuildDate != "unknown" {
		t.Fatalf("BuildDate = %q, want unknown", BuildDate)
	}
}

func TestCurrentUsesRuntimeVersion(t *testing.T) {
	if got := Current().GoVersion; got != runtime.Version() {
		t.Fatalf("GoVersion = %q, want %q", got, runtime.Version())
	}
}

func TestStringIsStable(t *testing.T) {
	info := Info{Version: "v1.2.3", Commit: "abc123", BuildDate: "2026-01-02T03:04:05Z", GoVersion: "go1.27.1"}
	want := "uPaste v1.2.3\ncommit: abc123\nbuilt: 2026-01-02T03:04:05Z\ngo: go1.27.1\n"
	if got := info.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if !strings.HasPrefix(info.String(), "uPaste v1.2.3\n") {
		t.Fatalf("String() does not start with a stable version line: %q", info.String())
	}
}
