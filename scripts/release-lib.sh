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

# release_require_clean_tree refuses to package uncommitted tracked changes: a
# release artifact claims a specific commit, so the source it was built from
# must be that commit. Ignored build output and untracked files never enter the
# artifact and are therefore irrelevant. RELEASE_ALLOW_DIRTY=1 is the explicit
# operator override for deliberate local experiments.
release_require_clean_tree() {
  [[ "${RELEASE_ALLOW_DIRTY:-}" == "1" ]] && return 0
  local status
  if ! status=$(git status --porcelain --untracked-files=no 2>/dev/null); then
    release_die "git status failed; refusing to package without verified repository state"
  fi
  if [[ -n "$status" ]]; then
    printf '%s\n' "$status" >&2
    release_die "working tree has uncommitted tracked changes; commit them or set RELEASE_ALLOW_DIRTY=1"
  fi
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
