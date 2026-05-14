#!/usr/bin/env bash
set -Eeuo pipefail

TRYBOX_REPO_USED="${TRYBOX_REPO:-${FIREFOX_REPO:-${HOME:-}/firefox}}"
TRYBOX_TARGET_USED="${TRYBOX_TARGET:-macos15-arm64}"

export TRYBOX_REPO="$TRYBOX_REPO_USED"

[[ -d "$TRYBOX_REPO_USED" ]]
[[ -x "$TRYBOX_REPO_USED/mach" ]]
command -v trybox >/dev/null

trybox run --target "$TRYBOX_TARGET_USED" -- bash -lc 'cd "$HOME/trybox" && ./mach build && ./mach marionette-test --tag os_integration && ./mach mochitest --tag os_integration && ./mach xpcshell-test --tag os_integration'
