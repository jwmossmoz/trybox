#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP=""
TRYBOX_REPO_USED="${TRYBOX_REPO:-${FIREFOX_REPO:-${HOME:-}/firefox}}"
TRYBOX_TARGET_USED="${TRYBOX_TARGET:-macos15-arm64}"

cleanup() {
  if [[ -n "$TMP" ]]; then
    rm -rf "$TMP"
  fi
}

fail() {
  printf 'firefox-os-integration: %s\n' "$*" >&2
  exit 1
}

trap cleanup EXIT

if ! command -v trybox >/dev/null 2>&1; then
  command -v go >/dev/null 2>&1 || fail 'trybox is not on PATH and go is not available to build it'
  TMP="$(mktemp -d)"
  printf 'firefox-os-integration: trybox not on PATH; building local binary\n' >&2
  (cd "$ROOT" && go build -o "$TMP/trybox" ./cmd/trybox)
  export PATH="$TMP:$PATH"
fi

[[ -d "$TRYBOX_REPO_USED" ]] || fail "Firefox checkout not found: $TRYBOX_REPO_USED"
[[ -x "$TRYBOX_REPO_USED/mach" ]] || fail "Firefox mach is not executable: $TRYBOX_REPO_USED/mach"

export TRYBOX_REPO="$TRYBOX_REPO_USED"

printf 'firefox-os-integration: repo=%s\n' "$TRYBOX_REPO_USED" >&2
printf 'firefox-os-integration: target=%s\n' "$TRYBOX_TARGET_USED" >&2
printf 'firefox-os-integration: running build and os_integration suites through Trybox\n' >&2

trybox run --target "$TRYBOX_TARGET_USED" -- bash -lc 'cd "$HOME/trybox" && ./mach build && ./mach marionette-test --tag os_integration && ./mach mochitest --tag os_integration && ./mach xpcshell-test --tag os_integration'
