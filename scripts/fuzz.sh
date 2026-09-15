#!/usr/bin/env bash
# Bounded local/pre-release fuzz campaign. Not part of every CI run; ordinary
# CI runs the same targets' seed corpora via `go test`.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
duration="${FUZZTIME:-15s}"
cd "$root"

run_fuzz() {
  local package="$1" target="$2"
  echo "fuzz: $package $target ($duration)"
  go test -run '^$' -fuzz "^${target}$" -fuzztime "$duration" "$package"
}

run_fuzz ./internal/capability FuzzParseShareID
run_fuzz ./internal/capability FuzzOwnerTokenParsing
run_fuzz ./internal/abuse FuzzClientIP
run_fuzz ./internal/abuse FuzzRateKey
run_fuzz ./internal/httpapi FuzzDecodeBase64URL
run_fuzz ./internal/httpapi FuzzSanitizeFilename
run_fuzz ./internal/config FuzzParseCIDRs

echo "fuzz: bounded campaign passed"
