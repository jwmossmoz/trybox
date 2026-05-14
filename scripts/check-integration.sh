#!/usr/bin/env bash
set -Eeuo pipefail
set -x

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
TMP="$(cd "$TMP" && pwd -P)"
REAL_HOME="${HOME:-}"
TRYBOX_HOME="$TMP/home"
FIXTURE="$TMP/fixture-repo"
FIREFOX_REPO_USED=""

cleanup() {
  set +e
  if command -v trybox >/dev/null 2>&1; then
    if [[ -d "${FIXTURE:-}" ]]; then
      trybox destroy --target macos15-arm64 --repo "$FIXTURE" --json >/dev/null 2>&1
    fi
    if [[ -n "${FIREFOX_REPO_USED:-}" ]]; then
      trybox destroy --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >/dev/null 2>&1
    fi
  fi
  chmod -R u+w "$TMP" 2>/dev/null || true
  rm -rf "$TMP"
}

trap 'printf "integration check failed near line %s\n" "$LINENO" >&2' ERR
trap cleanup EXIT

command -v go
command -v git
command -v jq

mkdir -p "$TRYBOX_HOME" "$TMP/bin"
real_trybox_existed=0
if [[ -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  real_trybox_existed=1
fi

cd "$ROOT"
go test ./...
go build -o "$TMP/bin/trybox" ./cmd/trybox

export PATH="$TMP/bin:$PATH"
export HOME="$TRYBOX_HOME"

mkdir -p "$FIXTURE"
git -C "$FIXTURE" init -q
git -C "$FIXTURE" config user.email trybox-integration@example.invalid
git -C "$FIXTURE" config user.name "Trybox Integration"
printf 'ignored.log\n' >"$FIXTURE/.gitignore"
printf 'tracked\n' >"$FIXTURE/tracked.txt"
printf 'ignored\n' >"$FIXTURE/ignored.log"
git -C "$FIXTURE" add .gitignore tracked.txt
printf 'changed\n' >"$FIXTURE/tracked.txt"
printf 'untracked\n' >"$FIXTURE/untracked.txt"

trybox target list --json >"$TMP/targets.json"
jq empty "$TMP/targets.json"
jq -e 'any(.[]; .name == "macos15-arm64")' "$TMP/targets.json" >/dev/null

trybox status --target macos15-arm64 --repo "$FIXTURE" --json >"$TMP/status.json"
jq empty "$TMP/status.json"
jq -e --arg repo "$FIXTURE" '.vm.repo_root == $repo and .exists == false and .running == false' "$TMP/status.json" >/dev/null

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
  command -v tart
  if ! tart list 2>/dev/null | grep -q 'trybox-macos15-arm64-image'; then
    printf 'trybox-macos15-arm64-image is missing; create it before running the full integration check\n' >&2
    exit 1
  fi

  trybox run --target macos15-arm64 --repo "$FIXTURE" --json -- sh -lc 'pwd && test -d .git && printf vm-ok' >"$TMP/vm-run.json"
  jq empty "$TMP/vm-run.json"
  run_id="$(jq -r '.id' "$TMP/vm-run.json")"

  trybox logs >"$TMP/vm-latest-log.txt"
  grep -q vm-ok "$TMP/vm-latest-log.txt"

  trybox events "$run_id" --json >"$TMP/vm-events.json"
  jq empty "$TMP/vm-events.json"

  trybox history --limit 5 --json >"$TMP/vm-history.json"
  jq empty "$TMP/vm-history.json"

  trybox status --target macos15-arm64 --repo "$FIXTURE" --json >"$TMP/vm-status.json"
  jq empty "$TMP/vm-status.json"

  trybox view --target macos15-arm64 --repo "$FIXTURE" --vnc --json >"$TMP/vm-view.json"
  jq empty "$TMP/vm-view.json"
  jq -e '.display == "tart-vnc" and .client == "none" and (.url | startswith("vnc://"))' "$TMP/vm-view.json" >/dev/null

  trybox destroy --target macos15-arm64 --repo "$FIXTURE" --json >"$TMP/vm-destroy.json"
  jq empty "$TMP/vm-destroy.json"
fi

if [[ "${TRYBOX_INTEGRATION_FIREFOX:-}" == "1" ]]; then
  command -v tart
  tart list 2>/dev/null | grep -q 'trybox-macos15-arm64-image'
  FIREFOX_REPO_USED="${FIREFOX_REPO:-$REAL_HOME/firefox}"
  [[ -d "$FIREFOX_REPO_USED" ]]
  [[ -x "$FIREFOX_REPO_USED/mach" ]]

  mach_command="${TRYBOX_FIREFOX_MACH_COMMAND:-./mach --help >/dev/null && printf mach-ok}"
  trybox run --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json -- sh -lc "$mach_command" >"$TMP/firefox-run.json"
  jq empty "$TMP/firefox-run.json"

  trybox logs >"$TMP/firefox-log.txt"
  grep -q mach-ok "$TMP/firefox-log.txt"

  trybox status --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >"$TMP/firefox-status.json"
  jq empty "$TMP/firefox-status.json"

  trybox destroy --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >"$TMP/firefox-destroy.json"
  jq empty "$TMP/firefox-destroy.json"
fi

printf 'integration check passed\n'
