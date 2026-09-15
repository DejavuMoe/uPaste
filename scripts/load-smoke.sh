#!/usr/bin/env bash
# Bounded private-deployment load smoke. Verifies the service stays healthy,
# does not return 5xx under the configured process-local limits, exercises
# backpressure, and recovers immediately. Informational local/pre-release check.
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

binary="$work/upaste"
./scripts/build-production-binary.sh "$binary" >/dev/null
data_dir="$work/data"; mkdir -p "$data_dir"
base=$(( (RANDOM % 2000) + 50000 ))
app_port=$base
file_port=$((base + 1))

UPASTE_ADDR="127.0.0.1:$app_port" \
UPASTE_FILE_ADDR="127.0.0.1:$file_port" \
UPASTE_FILE_ORIGIN="http://127.0.0.1:$file_port" \
UPASTE_DATA_DIR="$data_dir" \
UPASTE_TRUSTED_PROXY_CIDRS="127.0.0.1/32" \
  "$binary" >"$work/server.log" 2>&1 &
server_pid=$!
for _ in $(seq 1 100); do curl -fsS "http://127.0.0.1:$app_port/healthz" >/dev/null 2>&1 && break; sleep 0.1; done
curl -fsS "http://127.0.0.1:$app_port/healthz" >/dev/null

app="http://127.0.0.1:$app_port"
json_field() { python3 -c "import json,sys; print(json.load(sys.stdin)$1)"; }

# Create a Text Share and a File Share sequentially before load.
text_json=$(curl -fsS -H 'Content-Type: application/json' -d '{"payload_kind":"TEXT","privacy_mode":"STANDARD","text":{"format":"PLAIN","content":"load smoke text"},"expires_at":null}' "$app/api/v1/shares")
text_id=$(json_field '["share"]["id"]' <<<"$text_json")
head -c $((2 << 20)) /dev/zero > "$work/load-file.bin"
file_json=$(curl -fsS -F 'metadata={"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null};type=application/json' -F "file=@$work/load-file.bin" "$app/api/v1/shares")
download_url=$(json_field '["share"]["file"]["download_url"]' <<<"$file_json")

statuses="$work/statuses"; : > "$statuses"
read_status() { curl -s -o /dev/null -w '%{http_code}' -H 'X-Forwarded-For: 198.51.100.10' "$1"; }
create_status() { curl -s -o /dev/null -w '%{http_code}' -H 'X-Forwarded-For: 198.51.100.11' -X POST -H 'Content-Type: application/json' -d "{\"payload_kind\":\"TEXT\",\"privacy_mode\":\"STANDARD\",\"text\":{\"format\":\"PLAIN\",\"content\":\"load create\"},\"expires_at\":null}" "$1"; }
upload_status() { curl -s -o /dev/null -w '%{http_code}' -H 'X-Forwarded-For: 198.51.100.12' -X POST -F 'metadata={"payload_kind":"FILE","privacy_mode":"STANDARD","expires_at":null};type=application/json' -F "file=@$1" "$2"; }
download_status() { curl -s -o /dev/null -w '%{http_code}' -H 'X-Forwarded-For: 198.51.100.13' "$1"; }

run_parallel() {
  local name="$1" count="$2"; shift 2
  local pids=()
  for _ in $(seq 1 "$count"); do
    (
      code=$("$@") || code=000
      printf '%s %s\n' "$name" "$code" >> "$statuses"
    ) &
    pids+=($!)
  done
  wait "${pids[@]}" 2>/dev/null || true
}

rss_before=$(awk '/VmRSS/ {print $2}' "/proc/$server_pid/status")

run_parallel read 40 read_status "$app/api/v1/shares/$text_id"
run_parallel upload 8 upload_status "$work/load-file.bin" "$app/api/v1/shares"
run_parallel create 10 create_status "$app/api/v1/shares"
run_parallel download 64 download_status "$download_url"

rss_after=$(awk '/VmRSS/ {print $2}' "/proc/$server_pid/status")
peak_after=$(awk '/VmHWM/ {print $2}' "/proc/$server_pid/status")

# Recovery: health endpoint and an existing Share read must still work.
health=$(curl -s -o /dev/null -w '%{http_code}' "$app/healthz")
recovery=$(curl -s -o /dev/null -w '%{http_code}' -H 'X-Forwarded-For: 198.51.100.14' "$app/api/v1/shares/$text_id")

kill "$server_pid"; wait "$server_pid" 2>/dev/null || true
server_pid=""

python3 - "$statuses" "$health" "$recovery" "$rss_before" "$rss_after" "$peak_after" <<'PY'
import collections, sys
path, health, recovery, rss_before, rss_after, peak = sys.argv[1:7]
counts = collections.Counter()
for line in open(path):
    name, code = line.split()
    counts[(name, code)] += 1
for key in sorted(counts):
    print(f"{key[0]} {key[1]}: {counts[key]}")
if health != "200" or recovery != "200":
    raise SystemExit(f"recovery failed: healthz={health} read={recovery}")
by_name = collections.defaultdict(set)
for (name, code), _ in counts.items():
    by_name[name].add(code)
    if code.startswith("5"):
        raise SystemExit(f"unexpected 5xx in {name}: {code}")
for name in ("read", "create", "upload", "download"):
    if name not in by_name or not any(c.startswith("2") for c in by_name[name]):
        raise SystemExit(f"no successful responses in {name}: {by_name.get(name)}")
for name in ("create", "upload", "download"):
    if not any(c == "429" for c in by_name.get(name, ())):
        raise SystemExit(f"{name} load did not exercise backpressure")
print(f"RSS before KiB: {rss_before}")
print(f"RSS after KiB: {rss_after}")
print(f"peak RSS KiB: {peak}")
PY

echo "load-smoke: bounded load smoke passed"
