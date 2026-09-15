#!/usr/bin/env bash
# Practical local performance baseline for release awareness. Machine-specific
# numbers are informational only and are not committed as thresholds.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

work=$(mktemp -d)
server_pid=""
cleanup() {
  if [[ -n "$server_pid" ]]; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "$work"
}
trap cleanup EXIT INT TERM

for tool in go curl python3; do
  command -v "$tool" >/dev/null 2>&1 || { echo "benchmark-local: missing $tool" >&2; exit 2; }
done

binary="$work/upaste"
./scripts/build-production-binary.sh "$binary" >/dev/null

data_dir="$work/data"
mkdir -p "$data_dir"
base=$(( (RANDOM % 2000) + 48000 ))
app_port=$base
file_port=$((base + 1))
log="$work/server.log"

UPASTE_ADDR="127.0.0.1:$app_port" \
UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
UPASTE_DATA_DIR="$data_dir" \
  "$binary" >"$log" 2>&1 &
server_pid=$!

for _ in $(seq 1 100); do
  curl -fsS "http://127.0.0.1:$app_port/healthz" >/dev/null 2>&1 && break
  sleep 0.1
done
curl -fsS "http://127.0.0.1:$app_port/healthz" >/dev/null

# timed OUT CURL_ARGS...
timed() {
  local out="$1"
  shift
  curl -sS -o "$out" -w '%{time_total}' "$@"
}

json_field() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)"; }

echo "=== uPaste local performance baseline ==="
echo "binary bytes: $(stat -c '%s' "$binary")"

echo "index seconds: $(timed /dev/null "http://127.0.0.1:$app_port/")"
echo "healthz seconds: $(timed /dev/null "http://127.0.0.1:$app_port/healthz")"

small_body='{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"benchmark text"},"expires_at":null}'
echo "small text create seconds: $(timed "$work/small-text.json" -X POST -H 'Content-Type: application/json' -d "$small_body" "http://127.0.0.1:$app_port/api/v1/shares")"
small_id=$(json_field '["share"]["id"]' < "$work/small-text.json")
echo "small text read seconds: $(timed /dev/null "http://127.0.0.1:$app_port/api/v1/shares/$small_id")"

python3 -c 'import json; print(json.dumps({"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"x"*1048576},"expires_at":None}))' > "$work/1mib.json"
echo "1 MiB text create seconds: $(timed "$work/1mib-text.json" -X POST -H 'Content-Type: application/json' --data-binary "@$work/1mib.json" "http://127.0.0.1:$app_port/api/v1/shares")"
big_text_id=$(json_field '["share"]["id"]' < "$work/1mib-text.json")
echo "1 MiB text read seconds: $(timed /dev/null "http://127.0.0.1:$app_port/api/v1/shares/$big_text_id")"

head -c $((4 << 10)) /dev/zero > "$work/small.bin"
echo "small file create seconds: $(timed "$work/small-file.json" -X POST -F 'metadata={"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null};type=application/json' -F "file=@$work/small.bin" "http://127.0.0.1:$app_port/api/v1/shares")"
small_file_url=$(json_field '["share"]["file"]["download_url"]' < "$work/small-file.json")
echo "small file download seconds: $(timed /dev/null "$small_file_url")"

head -c $((64 << 20)) /dev/zero > "$work/64mib.bin"
echo "64 MiB file create seconds: $(timed "$work/64mib-file.json" -X POST -F 'metadata={"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null};type=application/json' -F "file=@$work/64mib.bin" "http://127.0.0.1:$app_port/api/v1/shares")"
big_file_url=$(json_field '["share"]["file"]["download_url"]' < "$work/64mib-file.json")
echo "64 MiB file download seconds: $(timed /dev/null "$big_file_url")"

rss_kb=$(awk '/VmRSS/ {print $2}' "/proc/$server_pid/status" 2>/dev/null || echo unknown)
peak_kb=$(awk '/VmHWM/ {print $2}' "/proc/$server_pid/status" 2>/dev/null || echo unknown)
echo "server RSS KiB: $rss_kb"
echo "server peak RSS KiB: $peak_kb"
echo "=== baseline complete ==="
