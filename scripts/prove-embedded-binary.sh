#!/usr/bin/env bash
# Proves the production executable is self-contained: the binary is copied
# alone into a fresh directory, run with a fresh data directory, and exercised
# over HTTP. No web/dist, repository checkout, or Node runtime is available.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
binary="${1:-$root/upaste}"
if [[ "$binary" != /* ]]; then
  binary="$PWD/$binary"
fi
if [[ ! -x "$binary" ]]; then
  echo "prove-embedded-binary: executable not found: $binary" >&2
  echo "build it first with scripts/build-production-binary.sh" >&2
  exit 2
fi

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

cp "$binary" "$work/upaste"
mkdir -p "$work/data"
base=$(( (RANDOM % 1000) + 44000 ))
app_port=$base
file_port=$((base + 1))

fail() {
  echo "PROOF FAILED: $1" >&2
  if [[ -f "$work/server.log" ]]; then cat "$work/server.log" >&2; fi
  exit 1
}

# Run from the isolated directory so a relative web/dist could never be read.
cd "$work"
UPASTE_ADDR="127.0.0.1:$app_port" \
UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
UPASTE_DATA_DIR="$work/data" \
  ./upaste >"$work/server.log" 2>&1 &
pid=$!

ready=0
for _ in $(seq 1 100); do
  if curl -fsS "http://127.0.0.1:$app_port/healthz" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 0.1
done
[[ "$ready" -eq 1 ]] || fail "server did not become ready"

status() { curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$app_port$1"; }

html=$(curl -fsS "http://127.0.0.1:$app_port/") || fail "GET / failed"
grep -q 'id="root"' <<<"$html" || fail "index.html root element missing"
grep -q '/assets/' <<<"$html" || fail "index.html hashed asset references missing"

for path in "/" "/s/AAAAAAAAAAAAAAAAAAAAAA" "/manage/AAAAAAAAAAAAAAAAAAAAAA"; do
  code=$(status "$path")
  [[ "$code" == "200" ]] || fail "expected 200 for $path, got $code"
done

for path in "/does-not-exist" "/admin" "/assets/does-not-exist.js" "/f/AAAAAAAAAAAAAAAAAAAAAA"; do
  code=$(status "$path")
  [[ "$code" == "404" ]] || fail "expected 404 for $path, got $code"
done

code=$(curl -s -o /dev/null -w '%{http_code}' -X POST "http://127.0.0.1:$app_port/healthz")
[[ "$code" == "405" ]] || fail "expected 405 for POST /healthz, got $code"

asset=$(grep -oE '/assets/[A-Za-z0-9._-]+' <<<"$html" | head -n 1)
[[ -n "$asset" ]] || fail "no hashed asset reference discovered"
asset_headers=$(curl -fsS -D - -o "$work/asset" "http://127.0.0.1:$app_port$asset")
grep -qi '^cache-control: *public, max-age=31536000, immutable' <<<"$asset_headers" || fail "asset missing immutable cache policy"
grep -qiE '^content-type: *(text/javascript|text/css)' <<<"$asset_headers" || fail "asset has an unexpected content type"
[[ -s "$work/asset" ]] || fail "asset body is empty"

code=$(status "/api/v1/shares/AAAAAAAAAAAAAAAAAAAAAA")
[[ "$code" == "404" ]] || fail "expected API 404, got $code"
code=$(status "/raw/AAAAAAAAAAAAAAAAAAAAAA")
[[ "$code" == "404" ]] || fail "expected raw 404, got $code"

file_root_code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$file_port/")
[[ "$file_root_code" == "404" ]] || fail "expected file listener root 404, got $file_root_code"
file_health_code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$file_port/healthz")
[[ "$file_health_code" == "404" ]] || fail "expected file listener /healthz 404, got $file_health_code"

echo "isolated embedded-binary proof passed (app=$app_port file=$file_port)"
