#!/usr/bin/env bash
# Static validation for native deployment and two-origin reverse-proxy examples.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$root"

fail() {
  echo "check-deployment: $*" >&2
  exit 1
}

require_file() {
  [[ -f "$1" ]] || fail "missing required deployment file: $1"
}

require_line() {
  local file="$1" pattern="$2"
  grep -Eq "$pattern" "$file" || fail "$file is missing required pattern: $pattern"
}

forbid_line() {
  local file="$1" pattern="$2"
  if grep -Eq "$pattern" "$file"; then
    fail "$file contains forbidden pattern: $pattern"
  fi
}

service=deploy/systemd/upaste.service
env_example=deploy/upaste.env.example
nginx=deploy/nginx/upaste.conf.example
caddy=deploy/caddy/Caddyfile.example
release_workflow=.github/workflows/release.yml

for file in "$service" "$env_example" "$nginx" "$caddy" "$release_workflow"; do
  require_file "$file"
done

# systemd service model and hardening.
require_line "$service" '^User=upaste$'
require_line "$service" '^Group=upaste$'
require_line "$service" '^EnvironmentFile=/etc/upaste/upaste\.env$'
require_line "$service" '^ExecStart=/usr/local/bin/upaste$'
require_line "$service" '^WorkingDirectory=/var/lib/upaste$'
require_line "$service" '^Restart=on-failure$'
require_line "$service" '^KillSignal=SIGTERM$'
require_line "$service" '^UMask=0077$'
require_line "$service" '^NoNewPrivileges=true$'
require_line "$service" '^PrivateTmp=true$'
require_line "$service" '^ProtectSystem=strict$'
require_line "$service" '^ProtectHome=true$'
require_line "$service" '^CapabilityBoundingSet=$'
require_line "$service" '^AmbientCapabilities=$'
require_line "$service" '^RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6$'
require_line "$service" '^ReadWritePaths=/var/lib/upaste$'
forbid_line "$service" '^User=root$'

# Environment example: loopback listeners, distinct File origin, loopback-only trust.
require_line "$env_example" '^UPASTE_DEPLOYMENT_MODE=private$'
require_line "$env_example" '^UPASTE_ADDR=127\.0\.0\.1:8080$'
require_line "$env_example" '^UPASTE_FILE_ADDR=127\.0\.0\.1:8081$'
require_line "$env_example" '^UPASTE_FILE_ORIGIN=https://files\.example\.com$'
require_line "$env_example" '^UPASTE_DATA_DIR=/var/lib/upaste$'
require_line "$env_example" '^UPASTE_TRUSTED_PROXY_CIDRS=127\.0\.0\.1/32,::1/128$'
forbid_line "$env_example" '^UPASTE_TRUSTED_PROXY_CIDRS=.*0\.0\.0\.0/0'
forbid_line "$env_example" '^UPASTE_TRUSTED_PROXY_CIDRS=.*::/0'
forbid_line "$env_example" '^UPASTE_ADMIN_TOKEN=up_a1_[A-Za-z0-9_-]{43}$'
forbid_line "$env_example" '^UPASTE_ADMIN_COOKIE_SECURE=false$'

# The shipped environment example must keep documenting the complete public-mode
# surface, otherwise an operator cannot configure the supported deployment.
require_line "$env_example" '^# UPASTE_PUBLIC_DEFAULT_TTL='
require_line "$env_example" '^# UPASTE_PUBLIC_MAX_TTL='
require_line "$env_example" '^# UPASTE_CHALLENGE_PROVIDER=cap$'
require_line "$env_example" '^# UPASTE_CAP_ENDPOINT='
require_line "$env_example" '^# UPASTE_CAP_SITE_KEY='
require_line "$env_example" '^# UPASTE_CAP_SECRET_KEY='
require_line "$env_example" '^# UPASTE_CHALLENGE_PROVIDER=turnstile$'
require_line "$env_example" '^# UPASTE_TURNSTILE_SITE_KEY='
require_line "$env_example" '^# UPASTE_TURNSTILE_SECRET_KEY='
require_line "$env_example" '^# UPASTE_TURNSTILE_HOSTNAME='
require_line "$env_example" '^# UPASTE_ADMIN_TOKEN='
require_line "$env_example" '^# UPASTE_ADMIN_COOKIE_SECURE=true$'

# Release workflow permissions and publication policy: validation is always
# read-only, and only a real v* tag push may reach the draft release job.
require_line "$release_workflow" '^permissions:$'
require_line "$release_workflow" '^  contents: read$'
forbid_line "$release_workflow" '^  contents: write$'
require_line "$release_workflow" "^ *if: github\\.event_name == 'push'$"
require_line "$release_workflow" '^ *--draft'
require_line "$release_workflow" '^ *gh release create '
write_grants=$(grep -c '^      contents: write$' "$release_workflow")
[[ "$write_grants" == "1" ]] || fail "$release_workflow must grant contents: write exactly once, found $write_grants"

# Nginx: two server names, two loopback upstreams, large body/timeouts, no CORS.
require_line "$nginx" 'server_name paste\.example\.com;'
require_line "$nginx" 'server_name files\.example\.com;'
require_line "$nginx" 'proxy_pass http://127\.0\.0\.1:8080;'
require_line "$nginx" 'proxy_pass http://127\.0\.0\.1:8081;'
require_line "$nginx" 'client_max_body_size 70m;'
require_line "$nginx" 'proxy_request_buffering off;'
require_line "$nginx" 'proxy_send_timeout 600s;'
require_line "$nginx" 'proxy_read_timeout 600s;'
require_line "$nginx" 'X-Forwarded-For'
require_line "$nginx" 'X-Forwarded-Proto'
forbid_line "$nginx" 'Access-Control-Allow-Origin'
forbid_line "$nginx" 'proxy_pass http://127\.0\.0\.1:8080/f'
forbid_line "$nginx" 'location /f/.*8080'

# Caddy: distinct hosts and loopback upstreams, no CORS.
require_line "$caddy" 'paste\.example\.com'
require_line "$caddy" 'files\.example\.com'
require_line "$caddy" '127\.0\.0\.1:8080'
require_line "$caddy" '127\.0\.0\.1:8081'
forbid_line "$caddy" 'Access-Control-Allow-Origin'

# If the web servers are present, syntax-check the examples without installing them.
if command -v caddy >/dev/null 2>&1; then
  caddy validate --config "$caddy" --adapter caddyfile >/dev/null || fail "caddy validate failed for $caddy"
fi

# Structural systemd validation. Substitute a local ExecStart path because CI
# and development hosts do not install /usr/local/bin/upaste.
if command -v systemd-analyze >/dev/null 2>&1; then
  tmpdir=$(mktemp -d)
  trap 'rm -rf "$tmpdir"' EXIT
  sed -e 's#^ExecStart=.*#ExecStart=/bin/true#
s#^User=.*#User=root#
s#^Group=.*#Group=root#
s#^EnvironmentFile=.*#EnvironmentFile=-/dev/null#
s#^WorkingDirectory=.*#WorkingDirectory=/#
s#^ReadWritePaths=.*#ReadWritePaths=/tmp#' "$service" > "$tmpdir/upaste.service"
  if ! systemd-analyze verify "$tmpdir/upaste.service" >/dev/null 2>&1; then
    systemd-analyze verify "$tmpdir/upaste.service" >&2 || true
    fail "systemd-analyze verify failed"
  fi
else
  echo "check-deployment: systemd-analyze not available; string checks only" >&2
fi

# Release version parser contract.
source "$root/scripts/release-lib.sh"
for valid in v1.0.0 v0.0.0-test v2.3.4-rc.1 v1.0.0+build.2 v1.0.0-rc.1+build.2; do
  release_version_is_valid "$valid" || fail "version parser rejected valid version: $valid"
done
for invalid in 1.0.0 v01.0.0 v1.0 v1.0.0- v1.0.0_rc v1.0.0-rc..1; do
  if release_version_is_valid "$invalid"; then
    fail "version parser accepted invalid version: $invalid"
  fi
done

# Clean-tree guard contract: packaging refuses uncommitted tracked changes and
# honors the explicit operator override.
guard_dir=$(mktemp -d)
guard_log="$guard_dir/guard.log"
if ! (
  cd "$guard_dir" || exit 1
  git init -q . &&
    printf 'tracked\n' > tracked.txt &&
    git add tracked.txt &&
    git -c user.email=check@example.invalid -c user.name=check -c commit.gpgsign=false commit -qm init &&
    release_require_clean_tree &&
    printf 'modified\n' >> tracked.txt &&
    ! ( release_require_clean_tree ) 2>/dev/null &&
    RELEASE_ALLOW_DIRTY=1 release_require_clean_tree
) >"$guard_log" 2>&1; then
  cat "$guard_log" >&2
  rm -rf "$guard_dir"
  fail "clean-tree packaging guard does not accept clean state or reject dirty tracked changes"
fi
rm -rf "$guard_dir"

echo "check-deployment: deployment examples, release workflow policy, clean-tree guard, systemd unit, and version parser validated"
