#!/usr/bin/env bash
# Verifies a packaged release: checksums, archive shape, embedded metadata,
# AArch64 identity, amd64 runtime behavior, restart persistence, and
# deterministic amd64 archive rebuild. It never creates a tag or release.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/release-lib.sh"
export RELEASE_TOOL=verify-release

version="${1:-}"
release_dir="${2:-$root/release}"
release_require_version "$version"

for tool in git curl tar gzip sha256sum python3 strings; do
  command -v "$tool" >/dev/null 2>&1 || release_die "required tool not found: $tool"
done

[[ -d "$release_dir" ]] || release_die "release directory not found: $release_dir"
release_dir=$(cd "$release_dir" && pwd)

amd64_archive="$release_dir/upaste-${version}-linux-amd64.tar.gz"
arm64_archive="$release_dir/upaste-${version}-linux-arm64.tar.gz"
[[ -f "$amd64_archive" ]] || release_die "missing archive: $amd64_archive"
[[ -f "$arm64_archive" ]] || release_die "missing archive: $arm64_archive"
[[ -f "$release_dir/SHA256SUMS" ]] || release_die "missing checksum file: $release_dir/SHA256SUMS"

commit="${COMMIT:-}"
if [[ -z "$commit" ]]; then
  commit=$(cd "$root" && git rev-parse --verify "HEAD^{commit}") || release_die "cannot resolve HEAD commit"
fi
[[ "$commit" =~ ^[0-9a-f]{40}$ ]] || release_die "COMMIT is not a full lowercase Git SHA: $commit"

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

fail() { release_die "$*"; }

# 1. Checksums.
(
  cd "$release_dir"
  sha256sum --check --strict SHA256SUMS >/dev/null
)

# 2. Archive shape and AArch64/metadata inspection.
mkdir -p "$work/arm64" "$work/amd64"
tar -xzf "$arm64_archive" -C "$work/arm64"
tar -xzf "$amd64_archive" -C "$work/amd64"

arm64_top="upaste-${version}-linux-arm64"
amd64_top="upaste-${version}-linux-amd64"
arm64_dir="$work/arm64/$arm64_top"
amd64_dir="$work/amd64/$amd64_top"
[[ -x "$arm64_dir/upaste" ]] || fail "arm64 archive is missing an executable upaste"
[[ -x "$amd64_dir/upaste" ]] || fail "amd64 archive is missing an executable upaste"

for expected in README.md DEPLOYMENT.md upaste.service upaste.env.example nginx.conf.example Caddyfile.example; do
  [[ -f "$arm64_dir/$expected" ]] || fail "arm64 archive is missing $expected"
  [[ -f "$amd64_dir/$expected" ]] || fail "amd64 archive is missing $expected"
done

arch_checked=0
if command -v file >/dev/null 2>&1; then
  file "$arm64_dir/upaste" | grep -qi 'aarch64' || fail "file(1) does not identify arm64 artifact as AArch64"
  arch_checked=1
fi
if command -v readelf >/dev/null 2>&1; then
  readelf -h "$arm64_dir/upaste" | grep -qi 'AArch64' || fail "readelf does not identify arm64 artifact as AArch64"
  arch_checked=1
fi
[[ "$arch_checked" -eq 1 ]] || fail "neither file(1) nor readelf is available for arm64 inspection"

for binary in "$arm64_dir/upaste" "$amd64_dir/upaste"; do
  strings "$binary" | grep -Fq "$version" || fail "$(basename "$binary") does not embed version $version"
  strings "$binary" | grep -Fq "$commit" || fail "$(basename "$binary") does not embed commit $commit"
  strings "$binary" | grep -Fq 'id="root"' || fail "$(basename "$binary") does not embed the frontend index"
done

# 3. amd64 version output.
version_output=$("$amd64_dir/upaste" --version)
grep -Fq "uPaste $version" <<<"$version_output" || fail "packaged binary version output does not match $version: $version_output"
grep -Fq "commit: $commit" <<<"$version_output" || fail "packaged binary commit output does not match $commit: $version_output"
grep -Fq "go: go" <<<"$version_output" || fail "packaged binary does not report its Go version: $version_output"

# 4. Runtime proof and restart persistence.
data_dir="$work/data"
mkdir -p "$data_dir"
base=$(( (RANDOM % 1000) + 45000 ))
app_port=$base
file_port=$((base + 1))
log_file="$work/server.log"
sample_file="$work/sample.bin"
downloaded_file="$work/downloaded.bin"
printf 'phase9 packaged file body' > "$sample_file"

start_server() {
  UPASTE_ADDR="127.0.0.1:$app_port" \
  UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
  UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
  UPASTE_DATA_DIR="$data_dir" \
    "$amd64_dir/upaste" >"$log_file" 2>&1 &
  pid=$!
  for _ in $(seq 1 100); do
    if curl -fsS "http://127.0.0.1:$app_port/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  fail "packaged server did not become ready"
}

stop_server() {
  [[ -n "$pid" ]] || return 0
  kill -TERM "$pid"
  local waited=0
  while kill -0 "$pid" 2>/dev/null; do
    waited=$((waited + 1))
    [[ "$waited" -gt 200 ]] && fail "packaged server did not exit after SIGTERM"
    sleep 0.1
  done
  wait "$pid"
  pid=""
}

start_server

curl -fsS "http://127.0.0.1:$app_port/" | grep -Fq 'id="root"' || fail "embedded frontend did not load"
curl -fsS "http://127.0.0.1:$app_port/healthz" | grep -Fq '"status":"ok"' || fail "healthz did not respond"

text_json=$(curl -fsS -H 'Content-Type: application/json' \
  -d '{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"phase9 persistence text"},"expires_at":null}' \
  "http://127.0.0.1:$app_port/api/v1/shares")
text_id=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["share"]["id"])' <<<"$text_json")
text_token=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["owner_token"])' <<<"$text_json")
[[ "$text_id" != "" && "$text_token" != "" ]] || fail "Standard Text create did not return id/owner token"

file_json=$(curl -fsS \
  -F 'metadata={"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null};type=application/json' \
  -F "file=@$sample_file;filename=sample.bin;type=application/octet-stream" \
  "http://127.0.0.1:$app_port/api/v1/shares")
file_id=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["share"]["id"])' <<<"$file_json")
download_url=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["share"]["file"]["download_url"])' <<<"$file_json")
[[ "$download_url" == "http://127.0.0.1:$file_port/f/$file_id" ]] || fail "File download_url is not on the configured separate File origin: $download_url"

curl -fsS "$download_url" -o "$downloaded_file"
cmp "$sample_file" "$downloaded_file" || fail "File download bytes did not match upload"
app_file_status=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$app_port/f/$file_id")
[[ "$app_file_status" == "404" ]] || fail "application listener exposed /f/: got $app_file_status"

stop_server
if grep -q '"level":"ERROR"' "$log_file"; then
  cat "$log_file" >&2
  fail "clean server run logged an ERROR"
fi

# Restart with the same data directory and verify persistent state.
start_server
curl -fsS "http://127.0.0.1:$app_port/api/v1/shares/$text_id" | grep -Fq 'phase9 persistence text' || fail "Standard Text Share did not survive restart"
curl -fsS "$download_url" -o "$downloaded_file"
cmp "$sample_file" "$downloaded_file" || fail "File Share did not survive restart"
stop_server

# 5. Deterministic amd64 rebuild from identical inputs.
rebuild_dir="$work/rebuild"
COMMIT="$commit" "$root/scripts/package-release.sh" "$version" "$rebuild_dir" >"$work/rebuild.log" 2>&1
first_sum=$(sha256sum "$amd64_archive" | awk '{print $1}')
second_sum=$(sha256sum "$rebuild_dir/upaste-${version}-linux-amd64.tar.gz" | awk '{print $1}')
[[ "$first_sum" == "$second_sum" ]] || fail "amd64 archive is not deterministic: $first_sum != $second_sum"

echo "verify-release: checksums, archive shape, metadata, AArch64 identity, amd64 runtime, restart persistence, and deterministic rebuild passed"
echo "verify-release: deterministic amd64 sha256=$first_sum"
