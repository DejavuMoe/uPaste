#!/usr/bin/env bash
# Verifies a packaged release: checksum coverage, exact archive file set,
# shipped-file integrity against the claimed commit, embedded metadata,
# AArch64 identity, private- and public-mode amd64 runtime behavior including
# Superadmin governance and creation-anchored retention, restart persistence,
# and deterministic rebuilds of both archives. It never creates a tag or
# release.
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

# The artifact is always built from the checked-out commit, so it is verified
# against that same commit; a requested COMMIT that is not HEAD is refused.
commit=$(cd "$root" && release_resolve_build_commit)

work=$(mktemp -d)
pid=""
cap_pid=""
cleanup() {
  if [[ -n "$pid" ]]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
  fi
  if [[ -n "$cap_pid" ]]; then
    kill "$cap_pid" 2>/dev/null || true
    wait "$cap_pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

fail() { release_die "$*"; }

# 1. Checksums and checksum coverage.
(
  cd "$release_dir"
  sha256sum --check --strict SHA256SUMS >/dev/null
)
sum_names=$(awk '{print $2}' "$release_dir/SHA256SUMS" | sort)
expected_sums=$(printf 'upaste-%s-linux-amd64.tar.gz\nupaste-%s-linux-arm64.tar.gz\n' "$version" "$version" | sort)
[[ "$sum_names" == "$expected_sums" ]] || fail "SHA256SUMS does not cover exactly the two release archives"

# 2. Archive shape, shipped-file integrity, and AArch64/metadata inspection.
mkdir -p "$work/arm64" "$work/amd64"
tar -xzf "$arm64_archive" -C "$work/arm64"
tar -xzf "$amd64_archive" -C "$work/amd64"

arm64_top="upaste-${version}-linux-arm64"
amd64_top="upaste-${version}-linux-amd64"
arm64_dir="$work/arm64/$arm64_top"
amd64_dir="$work/amd64/$amd64_top"
[[ -x "$arm64_dir/upaste" ]] || fail "arm64 archive is missing an executable upaste"
[[ -x "$amd64_dir/upaste" ]] || fail "amd64 archive is missing an executable upaste"

for expected in README.md DEPLOYMENT.md BUILDINFO upaste.service upaste.env.example nginx.conf.example Caddyfile.example; do
  [[ -f "$arm64_dir/$expected" ]] || fail "arm64 archive is missing $expected"
  [[ -f "$amd64_dir/$expected" ]] || fail "amd64 archive is missing $expected"
done

# The archives ship exactly the documented file set, and every shipped
# document or example equals the version in the commit the artifact claims.
expected_entries=$(printf '%s\n' README.md DEPLOYMENT.md BUILDINFO upaste.service upaste.env.example nginx.conf.example Caddyfile.example upaste | sort)
compare_release_file() {
  local archived="$1" tracked="$2"
  git -C "$root" show "$commit:$tracked" | cmp -s - "$archived" || fail "$(basename "$archived") does not match $tracked at commit $commit"
}
for dir in "$amd64_dir" "$arm64_dir"; do
  actual_entries=$(ls -A "$dir" | sort)
  [[ "$actual_entries" == "$expected_entries" ]] || fail "archive contains an unexpected file set: $(printf '%s ' "$dir" "$actual_entries")"
  compare_release_file "$dir/README.md" README.md
  compare_release_file "$dir/DEPLOYMENT.md" docs/DEPLOYMENT.md
  compare_release_file "$dir/upaste.service" deploy/systemd/upaste.service
  compare_release_file "$dir/upaste.env.example" deploy/upaste.env.example
  compare_release_file "$dir/nginx.conf.example" deploy/nginx/upaste.conf.example
  compare_release_file "$dir/Caddyfile.example" deploy/caddy/Caddyfile.example
  for shipped in README.md DEPLOYMENT.md BUILDINFO upaste.service upaste.env.example nginx.conf.example Caddyfile.example; do
    if grep -Eq 'up_(a1|o1)_[A-Za-z0-9_-]{43}' "$dir/$shipped"; then
      fail "$shipped ships token-shaped secret material"
    fi
  done
done

for pair in "amd64:$amd64_dir" "arm64:$arm64_dir"; do
  arch="${pair%%:*}"
  dir="${pair#*:}"
  grep -Fxq "version=$version" "$dir/BUILDINFO" || fail "$arch BUILDINFO version does not match $version"
  grep -Fxq "commit=$commit" "$dir/BUILDINFO" || fail "$arch BUILDINFO commit does not match $commit"
  grep -Fxq "target=linux/$arch" "$dir/BUILDINFO" || fail "$arch BUILDINFO target does not match linux/$arch"
  grep -Eq '^buildDate=[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$' "$dir/BUILDINFO" || fail "$arch BUILDINFO buildDate is not deterministic UTC RFC3339"
  grep -Eq '^go=go[0-9]' "$dir/BUILDINFO" || fail "$arch BUILDINFO Go version is missing"
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
read -r app_port file_port < <(release_pick_ports)
log_file="$work/server.log"
sample_file="$work/sample.bin"
downloaded_file="$work/downloaded.bin"
printf 'phase9 packaged file body' > "$sample_file"

# start_server <data_dir> <log_file> [ENV=value ...]
start_server() {
  local data_dir="$1" log_file="$2"
  shift 2
  env "$@" \
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

# expect_startup_failure <description> <expected-log-fragment> <ENV=value ...>:
# the packaged binary must refuse a configuration that violates a fail-closed
# contract, for the stated reason.
expect_startup_failure() {
  local description="$1" fragment="$2"
  shift 2
  if env "$@" UPASTE_DATA_DIR="$work/startup-data" "$amd64_dir/upaste" >"$work/startup.log" 2>&1; then
    fail "packaged binary started despite $description"
  fi
  grep -Fq "$fragment" "$work/startup.log" || fail "packaged binary refused $description for an unexpected reason"
}

start_server "$data_dir" "$log_file"

index_headers=$(curl -fsS -D - -o "$work/index.html" "http://127.0.0.1:$app_port/")
grep -Fq 'id="root"' "$work/index.html" || fail "embedded frontend did not load"
grep -qi '^cache-control: *no-store' <<<"$index_headers" || fail "index.html is not served no-store"
csp=$(grep -i '^content-security-policy:' <<<"$index_headers" || true)
grep -Fq "default-src 'self'" <<<"$csp" || fail "embedded frontend is missing the strict baseline CSP"
for forbidden in unsafe-inline unsafe-eval; do
  if grep -q "$forbidden" <<<"$csp"; then
    fail "private-mode CSP contains $forbidden"
  fi
done
asset=$(grep -oE '/assets/[A-Za-z0-9._-]+' "$work/index.html" | head -n 1)
[[ -n "$asset" ]] || fail "embedded index.html has no hashed asset reference"
asset_headers=$(curl -fsS -D - -o "$work/asset" "http://127.0.0.1:$app_port$asset")
grep -qiE '^cache-control: *public, max-age=31536000, immutable' <<<"$asset_headers" || fail "embedded asset lost its immutable cache policy"
[[ -s "$work/asset" ]] || fail "embedded asset body is empty"
curl -fsS "http://127.0.0.1:$app_port/healthz" | grep -Fq '"status":"ok"' || fail "healthz did not respond"

private_config=$(curl -fsS "http://127.0.0.1:$app_port/api/v1/config")
grep -Fq '"deployment_mode":"private"' <<<"$private_config" || fail "private default does not report private deployment mode"
grep -Fq '"admin_enabled":false' <<<"$private_config" || fail "private default unexpectedly reports Superadmin enabled"
admin_page_status=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$app_port/admin")
[[ "$admin_page_status" == "404" ]] || fail "private default exposed the admin SPA route: $admin_page_status"
admin_api_status=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$app_port/api/v1/admin/summary")
[[ "$admin_api_status" == "404" ]] || fail "private default exposed the admin API: $admin_api_status"

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
start_server "$data_dir" "$log_file"
curl -fsS "http://127.0.0.1:$app_port/api/v1/shares/$text_id" | grep -Fq 'phase9 persistence text' || fail "Standard Text Share did not survive restart"
curl -fsS "$download_url" -o "$downloaded_file"
cmp "$sample_file" "$downloaded_file" || fail "File Share did not survive restart"
stop_server

# 5. Public-mode and Superadmin runtime proof on the packaged binary.
cap_port=""
read -r cap_port _ < <(release_pick_ports)
python3 "$root/scripts/mock-cap-server.py" "$cap_port" >"$work/cap.log" 2>&1 &
cap_pid=$!
cap_token=""
for _ in $(seq 1 50); do
  if cap_token=$(curl -fsS -X POST "http://127.0.0.1:$cap_port/redeem" 2>/dev/null | python3 -c 'import json,sys; print(json.load(sys.stdin)["token"])' 2>/dev/null); then
    break
  fi
  sleep 0.1
done
[[ -n "$cap_token" ]] || fail "local Cap mock did not become ready"

admin_token="up_a1_$(printf 'A%.0s' {1..43})"
public_data="$work/public-data"
public_log="$work/public-server.log"
mkdir -p "$public_data"

# Public mode without a challenge provider or without a Superadmin token must
# fail closed before any listener is bound.
expect_startup_failure "public mode without a challenge provider" UPASTE_CHALLENGE_PROVIDER UPASTE_DEPLOYMENT_MODE=public
expect_startup_failure "public mode without a Superadmin token" UPASTE_ADMIN_TOKEN \
  UPASTE_DEPLOYMENT_MODE=public \
  UPASTE_CHALLENGE_PROVIDER=cap \
  UPASTE_CAP_ENDPOINT="http://127.0.0.1:$cap_port" \
  UPASTE_CAP_SITE_KEY=test-cap-site \
  UPASTE_CAP_SECRET_KEY=test-secret

start_server "$public_data" "$public_log" \
  UPASTE_DEPLOYMENT_MODE=public \
  UPASTE_PUBLIC_DEFAULT_TTL=24h \
  UPASTE_PUBLIC_MAX_TTL=168h \
  UPASTE_CHALLENGE_PROVIDER=cap \
  UPASTE_CAP_ENDPOINT="http://127.0.0.1:$cap_port" \
  UPASTE_CAP_SITE_KEY=test-cap-site \
  UPASTE_CAP_SECRET_KEY=test-secret \
  UPASTE_ADMIN_TOKEN="$admin_token"

public_config=$(curl -fsS "http://127.0.0.1:$app_port/api/v1/config")
grep -Fq '"deployment_mode":"public"' <<<"$public_config" || fail "public deployment mode is not reported"
grep -Fq '"admin_enabled":true' <<<"$public_config" || fail "public deployment does not report Superadmin enabled"
grep -Fq '"default_seconds":86400' <<<"$public_config" || fail "public default retention is not exposed"
grep -Fq '"max_seconds":604800' <<<"$public_config" || fail "public maximum retention is not exposed"
grep -Fq '"provider":"cap"' <<<"$public_config" || fail "public challenge provider is not exposed"
if grep -Fq 'test-secret' <<<"$public_config" || grep -Fq "$admin_token" <<<"$public_config"; then
  fail "public runtime config exposed secret material"
fi

challenge_required=$(curl -s -o "$work/challenge-required.json" -w '%{http_code}' \
  -H 'Content-Type: application/json' \
  -d '{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"phase12 packaged public text"},"expires_at":null}' \
  "http://127.0.0.1:$app_port/api/v1/shares")
[[ "$challenge_required" == "403" ]] || fail "public creation without a challenge returned $challenge_required"
grep -Fq '"code":"challenge_required"' "$work/challenge-required.json" || fail "public creation without a challenge did not report challenge_required"

challenge_failed=$(curl -s -o "$work/challenge-failed.json" -w '%{http_code}' \
  -H 'Content-Type: application/json' -H 'X-uPaste-Challenge: not-a-verifiable-token' \
  -d '{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"phase12 packaged public text"},"expires_at":null}' \
  "http://127.0.0.1:$app_port/api/v1/shares")
[[ "$challenge_failed" == "403" ]] || fail "public creation with an unverifiable challenge returned $challenge_failed"
grep -Fq '"code":"challenge_failed"' "$work/challenge-failed.json" || fail "server-side challenge verification did not reject an unverifiable token"

public_json=$(curl -fsS -H 'Content-Type: application/json' -H "X-uPaste-Challenge: $cap_token" \
  -d '{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"phase12 packaged public text"},"expires_at":null}' \
  "http://127.0.0.1:$app_port/api/v1/shares")
public_id=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["share"]["id"])' <<<"$public_json")
public_token=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["owner_token"])' <<<"$public_json")
[[ "$public_id" != "" && "$public_token" != "" ]] || fail "challenge-verified public create did not return id/owner token"

python3 -c '
import datetime, json, sys
share = json.loads(sys.argv[1])["share"]
if not share["expires_at"]:
    raise SystemExit("public Share has no expiration")
parse = lambda value: datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
if parse(share["expires_at"]) - parse(share["created_at"]) != datetime.timedelta(hours=24):
    raise SystemExit("public default expiration is not the configured default TTL")
' "$public_json" || fail "public creation did not receive the configured default finite retention"

over_body=$(python3 -c '
import datetime, json, sys
share = json.loads(sys.argv[1])["share"]
created = datetime.datetime.fromisoformat(share["created_at"].replace("Z", "+00:00"))
print(json.dumps({"expires_at": (created + datetime.timedelta(days=30)).isoformat().replace("+00:00", "Z")}))
' "$public_json")
patch_status=$(curl -s -o "$work/over-horizon.json" -w '%{http_code}' -X PATCH \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $public_token" -d "$over_body" \
  "http://127.0.0.1:$app_port/api/v1/shares/$public_id")
[[ "$patch_status" == "400" ]] || fail "public Share accepted an expiration beyond the creation-anchored maximum: $patch_status"

patch_status=$(curl -s -o "$work/clear-expiration.json" -w '%{http_code}' -X PATCH \
  -H 'Content-Type: application/json' -H "Authorization: Bearer $public_token" -d '{"expires_at":null}' \
  "http://127.0.0.1:$app_port/api/v1/shares/$public_id")
[[ "$patch_status" == "400" ]] || fail "public Share accepted a cleared expiration: $patch_status"

anonymous_admin=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$app_port/api/v1/admin/summary")
[[ "$anonymous_admin" == "401" ]] || fail "Superadmin API answered an anonymous request with $anonymous_admin"

login=$(curl -fsS -c "$work/admin-cookies.txt" -H 'Content-Type: application/json' \
  -d "{\"token\":\"$admin_token\"}" "http://127.0.0.1:$app_port/api/v1/admin/session")
csrf=$(python3 -c 'import json,sys; print(json.load(sys.stdin)["csrf"])' <<<"$login")
[[ -n "$csrf" ]] || fail "Superadmin login did not return a CSRF token"

listing=$(curl -fsS -b "$work/admin-cookies.txt" "http://127.0.0.1:$app_port/api/v1/admin/shares?limit=10")
grep -Fq "\"id\":\"$public_id\"" <<<"$listing" || fail "Superadmin listing did not include the created Share"
if grep -Fq 'phase12 packaged public text' <<<"$listing"; then
  fail "Superadmin listing exposed Share content instead of metadata only"
fi
if grep -Eq 'up_o1_[A-Za-z0-9_-]{43}' <<<"$listing"; then
  fail "Superadmin listing exposed owner capability material"
fi

no_csrf=$(curl -s -o /dev/null -w '%{http_code}' -b "$work/admin-cookies.txt" -X DELETE \
  "http://127.0.0.1:$app_port/api/v1/admin/shares/$public_id")
[[ "$no_csrf" == "403" ]] || fail "Superadmin delete without a CSRF header returned $no_csrf"

with_csrf=$(curl -s -o /dev/null -w '%{http_code}' -b "$work/admin-cookies.txt" -X DELETE \
  -H "X-uPaste-CSRF: $csrf" "http://127.0.0.1:$app_port/api/v1/admin/shares/$public_id")
[[ "$with_csrf" == "204" ]] || fail "Superadmin delete with a CSRF header returned $with_csrf"
deleted_status=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$app_port/api/v1/shares/$public_id")
[[ "$deleted_status" == "404" ]] || fail "Superadmin-deleted Share is still readable: $deleted_status"

stop_server
kill "$cap_pid" 2>/dev/null || true
wait "$cap_pid" 2>/dev/null || true
cap_pid=""
if grep -q '"level":"ERROR"' "$public_log"; then
  cat "$public_log" >&2
  fail "public-mode server run logged an ERROR"
fi

# 6. Deterministic rebuild of both archives from identical inputs.
rebuild_dir="$work/rebuild"
COMMIT="$commit" "$root/scripts/package-release.sh" "$version" "$rebuild_dir" >"$work/rebuild.log" 2>&1
first_sum=$(sha256sum "$amd64_archive" | awk '{print $1}')
second_sum=$(sha256sum "$rebuild_dir/upaste-${version}-linux-amd64.tar.gz" | awk '{print $1}')
[[ "$first_sum" == "$second_sum" ]] || fail "amd64 archive is not deterministic: $first_sum != $second_sum"
arm64_sum=$(sha256sum "$arm64_archive" | awk '{print $1}')
arm64_second=$(sha256sum "$rebuild_dir/upaste-${version}-linux-arm64.tar.gz" | awk '{print $1}')
[[ "$arm64_sum" == "$arm64_second" ]] || fail "arm64 archive is not deterministic: $arm64_sum != $arm64_second"

echo "verify-release: checksums, archive shape, shipped-file integrity, metadata, AArch64 identity, private/public runtime, Superadmin governance, restart persistence, and deterministic rebuild passed"
echo "verify-release: deterministic amd64 sha256=$first_sum"
echo "verify-release: deterministic arm64 sha256=$arm64_sum"
