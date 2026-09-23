#!/usr/bin/env bash
# Builds deterministic Linux amd64/arm64 release archives with embedded
# version metadata. Intended for local use and the tag-triggered release
# workflow. It never creates a Git tag or GitHub Release.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/release-lib.sh"
export RELEASE_TOOL=package-release

version="${1:-}"
output_dir="${2:-$root/release}"
release_require_version "$version"

cd "$root"

if [[ ! -f docs/DEPLOYMENT.md ]]; then
  release_die "docs/DEPLOYMENT.md is required for the release archive"
fi

command -v git >/dev/null 2>&1 || release_die "git is required to derive commit metadata"
release_require_clean_tree
commit=$(release_resolve_build_commit)

if [[ -n "${SOURCE_DATE_EPOCH:-}" ]]; then
  epoch="$SOURCE_DATE_EPOCH"
else
  epoch=$(git show -s --format=%ct "$commit") || release_die "cannot derive commit timestamp"
fi
[[ "$epoch" =~ ^[0-9]+$ ]] || release_die "SOURCE_DATE_EPOCH must be an integer"
build_date=$(date -u -d "@$epoch" +"%Y-%m-%dT%H:%M:%SZ")

echo "packaging uPaste $version"
echo "commit: $commit"
echo "built: $build_date"

# Build the frontend once and stage it for both architectures.
pnpm --dir web build
rm -rf internal/webapp/dist
mkdir -p internal/webapp/dist
cp -R web/dist/. internal/webapp/dist/

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

ldflags="-X github.com/DejavuMoe/uPaste/internal/buildinfo.Version=$version"
ldflags="$ldflags -X github.com/DejavuMoe/uPaste/internal/buildinfo.Commit=$commit"
ldflags="$ldflags -X github.com/DejavuMoe/uPaste/internal/buildinfo.BuildDate=$build_date"

build_arch() {
  local goarch="$1"
  local outdir="$work/$goarch"
  mkdir -p "$outdir"
  CGO_ENABLED=0 GOOS=linux GOARCH="$goarch" go build \
    -tags production \
    -trimpath \
    -ldflags "$ldflags" \
    -o "$outdir/upaste" \
    ./cmd/upaste
}

stage_arch() {
  local goarch="$1"
  local top="upaste-${version}-linux-${goarch}"
  local dir="$work/stage/$top"
  mkdir -p "$dir"
  install -m 0755 "$work/$goarch/upaste" "$dir/upaste"
  install -m 0644 README.md "$dir/README.md"
  install -m 0644 docs/DEPLOYMENT.md "$dir/DEPLOYMENT.md"
  install -m 0644 deploy/systemd/upaste.service "$dir/upaste.service"
  install -m 0644 deploy/upaste.env.example "$dir/upaste.env.example"
  install -m 0644 deploy/nginx/upaste.conf.example "$dir/nginx.conf.example"
  install -m 0644 deploy/caddy/Caddyfile.example "$dir/Caddyfile.example"

  cat > "$dir/BUILDINFO" <<BUILDINFO_EOF
version=$version
commit=$commit
buildDate=$build_date
go=$(go env GOVERSION)
target=linux/$goarch
BUILDINFO_EOF
  chmod 0644 "$dir/BUILDINFO"
}

build_arch amd64
build_arch arm64
stage_arch amd64
stage_arch arm64

mkdir -p "$output_dir"
output_dir=$(cd "$output_dir" && pwd)
rm -f "$output_dir"/upaste-*-linux-*.tar.gz "$output_dir/SHA256SUMS"

make_archive() {
  local goarch="$1"
  local top="upaste-${version}-linux-${goarch}"
  local artifact="$output_dir/${top}.tar.gz"
  tar --sort=name \
      --mtime="@$epoch" \
      --owner=0 --group=0 --numeric-owner \
      --format=gnu \
      -cf - -C "$work/stage" "$top" | gzip -n -9 > "$artifact"
}

make_archive amd64
make_archive arm64

(
  cd "$output_dir"
  sha256sum "upaste-${version}-linux-amd64.tar.gz" "upaste-${version}-linux-arm64.tar.gz" > SHA256SUMS
)

echo "release artifacts:"
ls -l "$output_dir"
