#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
data=$(mktemp -d)
cleanup() { kill "$api" "$vite" 2>/dev/null || true; wait "$api" "$vite" 2>/dev/null || true; rm -rf "$data"; }
trap cleanup EXIT INT TERM
UPASTE_ADDR=127.0.0.1:4180 UPASTE_FILE_ADDR=127.0.0.1:4181 UPASTE_FILE_ORIGIN=http://127.0.0.1:4181 UPASTE_DATA_DIR="$data" "$root/upaste" & api=$!
(cd "$root/web" && VITE_API_ORIGIN=http://127.0.0.1:4180 pnpm vite --host 127.0.0.1 --port 4173) & vite=$!
for _ in $(seq 1 60); do curl -fsS http://127.0.0.1:4180/healthz >/dev/null 2>&1 && curl -fsS http://127.0.0.1:4173 >/dev/null 2>&1 && break; sleep 1; done
curl -fsS http://127.0.0.1:4180/healthz >/dev/null
wait "$vite"
