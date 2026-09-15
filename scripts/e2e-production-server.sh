#!/usr/bin/env bash
# Playwright web server for the production embedded build. When UPASTE_BINARY
# is set (CI), that already-built binary is used; otherwise the script builds
# the frontend, stages it, and compiles a production binary in a temp dir.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
app_port=4174
file_port=4191
work=$(mktemp -d)
pid=""
cleanup() {
  if [[ -n "$pid" ]]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

binary="${UPASTE_BINARY:-}"
if [[ -z "$binary" ]]; then
  binary="$work/upaste"
  "$root/scripts/build-production-binary.sh" "$binary"
fi
if [[ "$binary" != /* ]]; then
  binary="$(cd "$(dirname "$binary")" && pwd)/$(basename "$binary")"
fi
[[ -x "$binary" ]] || { echo "e2e-production-server: executable not found: $binary" >&2; exit 2; }

mkdir -p "$work/data"
UPASTE_ADDR="127.0.0.1:$app_port" \
UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
UPASTE_DATA_DIR="$work/data" \
  "$binary" &
pid=$!

wait "$pid"
