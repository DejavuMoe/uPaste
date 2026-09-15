#!/usr/bin/env bash
# Builds the Vite production bundle, stages it inside internal/webapp, and
# compiles the single self-contained uPaste executable with -tags production.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
output="${1:-$root/upaste}"
if [[ "$output" != /* ]]; then
  output="$PWD/$output"
fi

cd "$root"

pnpm --dir web build

rm -rf internal/webapp/dist
mkdir -p internal/webapp/dist
cp -R web/dist/. internal/webapp/dist/

go build -tags production -o "$output" ./cmd/upaste

echo "production binary: $output"
