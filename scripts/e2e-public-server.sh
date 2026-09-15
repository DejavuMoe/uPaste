#!/usr/bin/env bash
# Playwright web server for public-mode qualification. The Turnstile widget
# script is intercepted inside the browser test; the server-side verifier uses
# the real canonical Cloudflare endpoint and is expected to fail closed for the
# fake test token.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
app_port=4175
file_port=4192
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
  "$root/scripts/build-production-binary.sh" "$binary" >/dev/null
fi
[[ -x "$binary" ]] || { echo "e2e-public-server: missing binary $binary" >&2; exit 2; }

mkdir -p "$work/data"
UPASTE_DEPLOYMENT_MODE=public \
UPASTE_PUBLIC_DEFAULT_TTL=24h \
UPASTE_PUBLIC_MAX_TTL=168h \
UPASTE_CHALLENGE_PROVIDER=turnstile \
UPASTE_TURNSTILE_SITE_KEY=1x00000000000000000000AA \
UPASTE_TURNSTILE_SECRET_KEY=test-secret \
UPASTE_TURNSTILE_HOSTNAME=127.0.0.1 \
UPASTE_CHALLENGE_TIMEOUT=2s \
UPASTE_ADMIN_TOKEN="up_a1_$(printf 'A%.0s' {1..43})" \
UPASTE_ADDR="127.0.0.1:$app_port" \
UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
UPASTE_DATA_DIR="$work/data" \
  "$binary" >"$work/server.log" 2>&1 &
pid=$!
wait "$pid"
