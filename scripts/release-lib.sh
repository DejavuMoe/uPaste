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
