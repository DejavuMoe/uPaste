#!/usr/bin/env bash
# Playwright web server for public Cap-mode qualification, backed by a
# deterministic local Cap Standalone mock. The real pinned Cap widget runs in
# Chromium and exchanges challenge/redeem traffic with this local mock.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
app_port=4176
file_port=4194
cap_port=4193
work=$(mktemp -d)
uplaste_pid=""
cap_pid=""
cleanup() {
  [[ -n "$uplaste_pid" ]] && { kill "$uplaste_pid" 2>/dev/null || true; wait "$uplaste_pid" 2>/dev/null || true; }
  [[ -n "$cap_pid" ]] && { kill "$cap_pid" 2>/dev/null || true; wait "$cap_pid" 2>/dev/null || true; }
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

python3 "$root/scripts/mock-cap-server.py" "$cap_port" >"$work/cap.log" 2>&1 &
cap_pid=$!

binary="${UPASTE_BINARY:-}"
if [[ -z "$binary" ]]; then
  binary="$work/upaste"
  "$root/scripts/build-production-binary.sh" "$binary" >/dev/null
fi
[[ -x "$binary" ]] || { echo "e2e-cap-server: missing binary $binary" >&2; exit 2; }

mkdir -p "$work/data"
UPASTE_DEPLOYMENT_MODE=public \
UPASTE_PUBLIC_DEFAULT_TTL=24h \
UPASTE_PUBLIC_MAX_TTL=168h \
UPASTE_CHALLENGE_PROVIDER=cap \
UPASTE_CAP_ENDPOINT="http://127.0.0.1:$cap_port" \
UPASTE_CAP_SITE_KEY=test-cap-site \
UPASTE_CAP_SECRET_KEY=test-secret \
UPASTE_CHALLENGE_TIMEOUT=5s \
UPASTE_ADMIN_TOKEN="up_a1_$(printf 'A%.0s' {1..43})" \
UPASTE_ADDR="127.0.0.1:$app_port" \
UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
UPASTE_DATA_DIR="$work/data" \
  "$binary" >"$work/upaste.log" 2>&1 &
uplaste_pid=$!
wait "$uplaste_pid"
