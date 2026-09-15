#!/usr/bin/env bash
# Complete deterministic pre-release qualification. Builds deterministic
# amd64/arm64 packages, runs the full Go and race suites, validates deployment
# examples, proves packaged runtime and restart behavior, and adds automated
# cold backup/restore and shutdown-under-load proofs.
#
# Longer fuzz campaigns and machine-specific performance baselines are explicit
# local/pre-release commands (see scripts/fuzz.sh and scripts/benchmark-local.sh)
# and are intentionally not part of every CI run.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/release-lib.sh"
export RELEASE_TOOL=qualify-release

version="${1:-}"
output_dir="${2:-$root/release}"
release_require_version "$version"
cd "$root"

for tool in go pnpm curl tar gzip sha256sum python3 cmp; do
  command -v "$tool" >/dev/null 2>&1 || release_die "required tool not found: $tool"
done

work=$(mktemp -d)
server_pid=""
slow_upload_pid=""
cleanup() {
  if [[ -n "$slow_upload_pid" ]]; then
    kill "$slow_upload_pid" 2>/dev/null || true
    wait "$slow_upload_pid" 2>/dev/null || true
  fi
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

fail() { release_die "$*"; }

wait_health() {
  local app_port="$1"
  for _ in $(seq 1 100); do
    if curl -fsS "http://127.0.0.1:$app_port/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done
  return 1
}

start_server() {
  local data_dir="$1" app_port="$2" file_port="$3" log_file="$4"
  UPASTE_ADDR="127.0.0.1:$app_port" \
  UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
  UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
  UPASTE_DATA_DIR="$data_dir" \
  UPASTE_TRUSTED_PROXY_CIDRS="127.0.0.1/32,::1/128" \
    "$binary" >"$log_file" 2>&1 &
  server_pid=$!
  wait_health "$app_port" || fail "packaged server did not become ready on port $app_port"
}

stop_server() {
  [[ -n "$server_pid" ]] || return 0
  kill -TERM "$server_pid"
  local waited=0
  while kill -0 "$server_pid" 2>/dev/null; do
    waited=$((waited + 1))
    [[ "$waited" -gt 200 ]] && fail "packaged server did not exit after SIGTERM"
    sleep 0.1
  done
  wait "$server_pid" || fail "packaged server exited non-zero after SIGTERM"
  server_pid=""
}

create_text() {
  local app_port="$1" content="$2"
  curl -fsS -H 'Content-Type: application/json' \
    -d "{\"payload_kind\":\"TEXT\",\"privacy_mode\":\"STANDARD\",\"text\":{\"format\":\"PLAIN\",\"content\":\"$content\"},\"expires_at\":null}" \
    "http://127.0.0.1:$app_port/api/v1/shares"
}

json_field() {
  python3 -c "import json,sys; value=json.load(sys.stdin); print(value$1)"
}

echo "== go vet ./... =="
go vet ./...

echo "== go test ./... =="
go test ./...

echo "== go test -race ./... =="
go test -race ./...

echo "== repeated race-sensitive package stress =="
go test -race -count=3 ./internal/share ./internal/database ./internal/maintenance ./internal/abuse ./internal/admin ./internal/adminapi ./internal/challenge
go test -race -count=3 ./internal/httpapi ./internal/fileapi ./internal/objectstore

echo "== deployment and systemd validation =="
./scripts/check-deployment.sh

echo "== deterministic package build and verification =="
./scripts/package-release.sh "$version" "$output_dir"
./scripts/verify-release.sh "$version" "$output_dir"

tar -xzf "$output_dir/upaste-${version}-linux-amd64.tar.gz" -C "$work"
binary="$work/upaste-${version}-linux-amd64/upaste"
[[ -x "$binary" ]] || fail "packaged amd64 binary missing"

echo "== automated cold backup and restore proof =="
backup_data="$work/backup-original"
mkdir -p "$backup_data"
read -r app_port file_port < <(release_pick_ports)

start_server "$backup_data" "$app_port" "$file_port" "$work/backup-server.log"
text_json=$(create_text "$app_port" "phase10 backup text")
text_id=$(json_field '["share"]["id"]' <<<"$text_json")
printf 'phase10 backup file body' > "$work/backup.bin"
file_json=$(curl -fsS \
  -F 'metadata={"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null};type=application/json' \
  -F "file=@$work/backup.bin;filename=backup.bin;type=application/octet-stream" \
  "http://127.0.0.1:$app_port/api/v1/shares")
file_id=$(json_field '["share"]["id"]' <<<"$file_json")
download_url=$(json_field '["share"]["file"]["download_url"]' <<<"$file_json")
stop_server

mkdir -p "$work/backup"
tar -C "$work" -czf "$work/backup/complete-data.tar.gz" "$(basename "$backup_data")"

# Mutate the original state after the backup to prove restore replaces it.
start_server "$backup_data" "$app_port" "$file_port" "$work/backup-mutate.log"
create_text "$app_port" "phase10 post-backup mutation" >/dev/null
stop_server

mkdir -p "$work/restored"
tar -C "$work/restored" -xzf "$work/backup/complete-data.tar.gz"
restored_data="$work/restored/$(basename "$backup_data")"
[[ -d "$restored_data" ]] || fail "restored data directory is missing"

start_server "$restored_data" "$app_port" "$file_port" "$work/restore-server.log"
restored_text=$(curl -fsS "http://127.0.0.1:$app_port/api/v1/shares/$text_id")
grep -Fq 'phase10 backup text' <<<"$restored_text" || fail "restored Text Share did not survive"
if grep -Fq 'phase10 post-backup mutation' <<<"$restored_text"; then
  fail "restore unexpectedly retained post-backup mutation"
fi
curl -fsS "$download_url" -o "$work/restored.bin"
cmp "$work/backup.bin" "$work/restored.bin" || fail "restored File bytes did not match"
# The restored File Share metadata must also remain available.
restored_file=$(curl -fsS "http://127.0.0.1:$app_port/api/v1/shares/$file_id")
grep -Fq '"payload_kind":"FILE"' <<<"$restored_file" || fail "restored File Share metadata did not survive"
stop_server

echo "== shutdown under active upload =="
shutdown_data="$work/shutdown-data"
mkdir -p "$shutdown_data"
read -r app_port file_port < <(release_pick_ports)
start_server "$shutdown_data" "$app_port" "$file_port" "$work/shutdown-server.log"
shutdown_text=$(create_text "$app_port" "phase10 shutdown text")
shutdown_text_id=$(json_field '["share"]["id"]' <<<"$shutdown_text")

head -c $((8 << 20)) /dev/zero > "$work/slow-large.bin"
curl -sS --limit-rate 200k \
  -F 'metadata={"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null};type=application/json' \
  -F "file=@$work/slow-large.bin;filename=slow-large.bin;type=application/octet-stream" \
  "http://127.0.0.1:$app_port/api/v1/shares" >"$work/slow-upload.out" 2>&1 &
slow_upload_pid=$!
sleep 1
[[ -n "$server_pid" ]] || fail "server disappeared before shutdown test"
kill -TERM "$server_pid"
waited=0
while kill -0 "$server_pid" 2>/dev/null; do
  waited=$((waited + 1))
  [[ "$waited" -gt 200 ]] && fail "server did not shut down under active upload"
  sleep 0.1
done
wait "$server_pid" || fail "server exited non-zero during shutdown under load"
server_pid=""
kill "$slow_upload_pid" 2>/dev/null || true
wait "$slow_upload_pid" 2>/dev/null || true
slow_upload_pid=""

# No finalized partial File object may remain after the interrupted upload.
final_objects=$(find "$shutdown_data/objects" -type f ! -name '.stage-*' 2>/dev/null | wc -l | tr -d ' ')
[[ "$final_objects" == "0" ]] || fail "shutdown left $final_objects finalized partial objects"

start_server "$shutdown_data" "$app_port" "$file_port" "$work/shutdown-restart.log"
restarted=$(curl -fsS "http://127.0.0.1:$app_port/api/v1/shares/$shutdown_text_id")
grep -Fq 'phase10 shutdown text' <<<"$restarted" || fail "restart after shutdown lost committed Text Share"
create_text "$app_port" "phase10 after shutdown restart" >/dev/null || fail "create after restart failed"
stop_server

echo "== filesystem permission boundary proof =="
permission_root="$work/permission-root"
mkdir -p "$permission_root/usr/local/bin" "$permission_root/etc/upaste" "$permission_root/var/lib/upaste"
install -m 0755 "$binary" "$permission_root/usr/local/bin/upaste"
printf 'UPASTE_ADDR=127.0.0.1:8080\nUPASTE_FILE_ADDR=127.0.0.1:8081\nUPASTE_DATA_DIR=%s\n' "$permission_root/var/lib/upaste" > "$permission_root/etc/upaste/upaste.env"
chmod 0555 "$permission_root" "$permission_root/usr/local/bin" "$permission_root/etc/upaste"
chmod 0700 "$permission_root/var/lib/upaste"
read -r app_port file_port < <(release_pick_ports)
pushd "$permission_root" >/dev/null
UPASTE_ADDR="127.0.0.1:$app_port" \
UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
UPASTE_DATA_DIR="$permission_root/var/lib/upaste" \
  "$permission_root/usr/local/bin/upaste" >"$work/permission-server.log" 2>&1 &
server_pid=$!
popd >/dev/null
wait_health "$app_port" || fail "server required write access outside the data directory"
stop_server
chmod -R u+w "$permission_root"
echo "filesystem permission boundary proof passed"

echo "qualify-release: complete pre-release qualification passed"
echo "qualify-release: version=$version commit=$(git rev-parse HEAD)"
