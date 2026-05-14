#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
TMP="$(cd "$TMP" && pwd -P)"
REAL_HOME="${HOME:-}"
TRYBOX_HOME="$TMP/home"
FIREFOX_REPO_USED="${FIREFOX_REPO:-$REAL_HOME/firefox}"

cleanup() {
  set +e
  if command -v trybox >/dev/null 2>&1; then
    if [[ -n "${FIREFOX_REPO_USED:-}" ]]; then
      trybox destroy --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >/dev/null 2>&1
    fi
  fi
  chmod -R u+w "$TMP" 2>/dev/null || true
  rm -rf "$TMP"
}

trap 'printf "integration check failed near line %s\n" "$LINENO" >&2' ERR
trap cleanup EXIT

command -v go >/dev/null
command -v git >/dev/null
command -v jq >/dev/null

mkdir -p "$TRYBOX_HOME" "$TMP/bin"
real_trybox_existed=0
if [[ -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  real_trybox_existed=1
fi

cd "$ROOT"
go test ./... >/dev/null
go build -o "$TMP/bin/trybox" ./cmd/trybox >/dev/null

export PATH="$TMP/bin:$PATH"
export HOME="$TRYBOX_HOME"

[[ -d "$FIREFOX_REPO_USED" ]]
[[ -x "$FIREFOX_REPO_USED/mach" ]]

trybox target list --json >"$TMP/targets.json"
jq empty "$TMP/targets.json"
jq -e 'any(.[]; .name == "macos15-arm64")' "$TMP/targets.json" >/dev/null

trybox status --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >"$TMP/status.json"
jq empty "$TMP/status.json"
jq -e --arg repo "$FIREFOX_REPO_USED" '.vm.repo_root == $repo' "$TMP/status.json" >/dev/null

trybox help run >"$TMP/help-run.txt"
grep -q TRYBOX_TARGET "$TMP/help-run.txt"

if trybox workspace >"$TMP/negative.out" 2>&1; then
  printf 'removed workspace command unexpectedly succeeded\n' >&2
  exit 1
fi
grep -q 'unknown command "workspace"' "$TMP/negative.out"

[[ -d "$HOME/.trybox" ]]
if [[ "$real_trybox_existed" == "0" && -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  printf 'integration check created state under the caller real HOME\n' >&2
  exit 1
fi

if [[ "${TRYBOX_SKIP_VM:-}" != "1" ]]; then
  command -v tart >/dev/null
  if ! tart list 2>/dev/null | grep -q 'trybox-macos15-arm64-image'; then
    printf 'trybox-macos15-arm64-image is missing; create it before running the full integration check\n' >&2
    exit 1
  fi

  mach_command="${TRYBOX_FIREFOX_MACH_COMMAND:-pwd && test -d .git && test -x ./mach && ./mach --help >/dev/null && printf firefox-ok}"
  trybox run --target macos15-arm64 --repo "$FIREFOX_REPO_USED" -- sh -lc "$mach_command"

  trybox logs >"$TMP/firefox-latest-log.txt"
  grep -q firefox-ok "$TMP/firefox-latest-log.txt"

  trybox history --limit 1 --json >"$TMP/vm-history.json"
  jq empty "$TMP/vm-history.json"
  run_id="$(jq -r '.[0].id' "$TMP/vm-history.json")"

  trybox events "$run_id" --json >"$TMP/vm-events.json"
  jq empty "$TMP/vm-events.json"

  trybox history --limit 5 --json >"$TMP/vm-history.json"
  jq empty "$TMP/vm-history.json"

  trybox status --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >"$TMP/vm-status.json"
  jq empty "$TMP/vm-status.json"

  trybox view --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --vnc | tee "$TMP/vnc-view.txt"
  grep -q '^display:   tart-vnc$' "$TMP/vnc-view.txt"
  grep -q '^client:    none$' "$TMP/vnc-view.txt"
  grep -q '^url:       vnc://' "$TMP/vnc-view.txt"
  grep -q '^open:      skipped$' "$TMP/vnc-view.txt"

  sleep "${TRYBOX_VIEW_PAUSE_SECONDS:-60}"

  trybox destroy --target macos15-arm64 --repo "$FIREFOX_REPO_USED"
fi

if [[ "${TRYBOX_INTEGRATION_FIREFOX:-}" == "1" ]]; then
  printf 'TRYBOX_INTEGRATION_FIREFOX is no longer needed; ~/firefox is the default integration repo\n' >&2
fi

printf 'integration check passed\n'
