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
