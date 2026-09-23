#!/usr/bin/env bash
# Shared helpers for release packaging scripts. This file is sourced, not run.

release_die() {
  echo "${RELEASE_TOOL:-release}: $*" >&2
  exit 1
}

release_version_is_valid() {
  local version="$1"
  [[ "$version" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-((0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(\.(0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*))?(\+[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$ ]]
}

release_require_version() {
  local version="$1"
  [[ -n "$version" ]] || release_die "usage: $0 VERSION [ARGS]"
  release_version_is_valid "$version" || release_die "VERSION must look like vMAJOR.MINOR.PATCH or vMAJOR.MINOR.PATCH-prerelease"
}

# release_require_clean_tree refuses to package a working tree that can change
# the build result while the artifact still claims HEAD. Untracked files are not
# exempt: an untracked file in a package directory joins `go build`, and
# untracked frontend or configuration files can feed Vite the same way. Ignored
# paths are exactly the declared generated outputs (`web/dist/`,
# `internal/webapp/dist/`, `release/`, `node_modules/`, Playwright reports,
# runtime data, the production binary), so they stay acceptable.
#
# RELEASE_ALLOW_DIRTY=1 is the explicit local-experiment override. The CI and
# release workflows never set it, and the real release path requires a clean
# source state.
release_require_clean_tree() {
  [[ "${RELEASE_ALLOW_DIRTY:-}" == "1" ]] && return 0
  local status
  if ! status=$(git status --porcelain 2>/dev/null); then
    release_die "git status failed; refusing to package without verified repository state"
  fi
  if [[ -n "$status" ]]; then
    printf '%s\n' "$status" >&2
    release_die "working tree is not a clean source state; commit or remove the listed entries, or set RELEASE_ALLOW_DIRTY=1 for a deliberate local experiment"
  fi
}

# release_resolve_build_commit prints the commit a release artifact may claim.
# Packaging always builds the checked-out tree, so a requested COMMIT that is
# not HEAD (even a real, resolvable historical commit) is refused instead of
# being recorded as provenance the artifact does not have.
release_resolve_build_commit() {
  local head_commit resolved
  head_commit=$(git rev-parse --verify "HEAD^{commit}") || release_die "cannot resolve HEAD to a commit"
  [[ "$head_commit" =~ ^[0-9a-f]{40}$ ]] || release_die "resolved HEAD is not a full lowercase Git SHA: $head_commit"
  if [[ -z "${COMMIT:-}" ]]; then
    printf '%s\n' "$head_commit"
    return 0
  fi
  resolved=$(git rev-parse --verify "${COMMIT}^{commit}") || release_die "COMMIT does not resolve to a commit: $COMMIT"
  [[ "$resolved" == "$head_commit" ]] || release_die "COMMIT $resolved does not match checked-out HEAD $head_commit; release packaging only builds the checked-out commit"
  printf '%s\n' "$head_commit"
}

# release_pick_ports prints two currently-free loopback ports. This avoids
# flaky random-port collisions in qualification scripts.
release_pick_ports() {
  python3 - <<'PY'
import socket
sockets = []
try:
    for _ in range(2):
        sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        sock.bind(("127.0.0.1", 0))
        sockets.append(sock)
    print(sockets[0].getsockname()[1], sockets[1].getsockname()[1])
finally:
    for sock in sockets:
        sock.close()
PY
}
