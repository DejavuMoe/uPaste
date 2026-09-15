// Package buildinfo exposes stable build metadata for operator diagnostics.
//
// Defaults identify an unstamped development build. Release tooling injects
// Version, Commit, and BuildDate with Go linker -X flags; no project version is
// hardcoded in source.
package buildinfo

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic release version, for example v1.0.0. Release
	// builds inject it; development builds report "devel".
	Version = "devel"
	// Commit is the full Git commit SHA for production builds.
	Commit = "unknown"
	// BuildDate is the deterministic UTC build timestamp for production builds.
	BuildDate = "unknown"
)

// Info is an immutable snapshot of build metadata.
type Info struct {
	Version   string
	Commit    string
	BuildDate string
	GoVersion string
}

// Current returns the metadata compiled into this binary.
func Current() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
	}
}

// String renders the stable operator-facing version output.
func (info Info) String() string {
	return fmt.Sprintf("uPaste %s\ncommit: %s\nbuilt: %s\ngo: %s\n", info.Version, info.Commit, info.BuildDate, info.GoVersion)
}
