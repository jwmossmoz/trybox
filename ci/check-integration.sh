#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
TMP="$(cd "$TMP" && pwd -P)"
REAL_HOME="${HOME:-}"
TRYBOX_HOME="$TMP/home"
TRYBOX_REPO_USED="${TRYBOX_REPO:-${FIREFOX_REPO:-$REAL_HOME/firefox}}"
TRYBOX_TARGET_USED="${TRYBOX_TARGET:-macos15-arm64}"

cleanup() {
  set +e
  if command -v trybox >/dev/null 2>&1; then
    if [[ -n "${TRYBOX_REPO_USED:-}" ]]; then
      TRYBOX_REPO="$TRYBOX_REPO_USED" TRYBOX_TARGET="$TRYBOX_TARGET_USED" trybox destroy --json >/dev/null 2>&1
    fi
  fi
  chmod -R u+w "$TMP" 2>/dev/null || true
  rm -rf "$TMP"
}

trap 'printf "integration check failed near line %s\n" "$LINENO" >&2' ERR
trap cleanup EXIT

printf 'integration: checking host tools\n' >&2
command -v go >/dev/null
command -v git >/dev/null
command -v jq >/dev/null

mkdir -p "$TRYBOX_HOME" "$TMP/bin"
real_trybox_existed=0
if [[ -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  real_trybox_existed=1
fi

cd "$ROOT"
printf 'integration: building local trybox\n' >&2
go test ./... >/dev/null
go build -o "$TMP/bin/trybox" ./cmd/trybox >/dev/null

export PATH="$TMP/bin:$PATH"
export HOME="$TRYBOX_HOME"
export TRYBOX_REPO="$TRYBOX_REPO_USED"
export TRYBOX_TARGET="$TRYBOX_TARGET_USED"

[[ -d "$TRYBOX_REPO_USED" ]]
[[ -x "$TRYBOX_REPO_USED/mach" ]]

printf 'integration: validating target and repo state\n' >&2
trybox target list --json >"$TMP/targets.json"
jq empty "$TMP/targets.json"
jq -e --arg target "$TRYBOX_TARGET_USED" 'any(.[]; .name == $target)' "$TMP/targets.json" >/dev/null

trybox status --json >"$TMP/status.json"
jq empty "$TMP/status.json"
jq -e --arg repo "$TRYBOX_REPO_USED" --arg target "$TRYBOX_TARGET_USED" '.vm.repo_root == $repo and .vm.target == $target' "$TMP/status.json" >/dev/null

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
  printf 'integration: checking Tart target image\n' >&2
  if ! trybox doctor --json >"$TMP/doctor.json" || ! jq -e 'all(.[]; .ok == true)' "$TMP/doctor.json" >/dev/null; then
    trybox doctor >&2 || true
    exit 1
  fi

  printf 'integration: running Firefox smoke command in the VM\n' >&2
  mach_command="${TRYBOX_FIREFOX_MACH_COMMAND:-pwd && test -d .git && test -x ./mach && ./mach --help >/dev/null && printf 'firefox-ok\\n'}"
  trybox run -- sh -lc "$mach_command"

  printf 'integration: verifying durable logs, history, and events\n' >&2
  trybox logs >"$TMP/firefox-latest-log.txt"
  grep -q firefox-ok "$TMP/firefox-latest-log.txt"

  trybox history --limit 1 --json >"$TMP/vm-history.json"
  jq empty "$TMP/vm-history.json"
  run_id="$(jq -r '.[0].id' "$TMP/vm-history.json")"

  trybox logs "$run_id" --json >"$TMP/vm-logs.json"
  jq empty "$TMP/vm-logs.json"
  jq -e --arg run_id "$run_id" '.run_id == $run_id and (.output | contains("firefox-ok")) and (.run.command_string | length > 0)' "$TMP/vm-logs.json" >/dev/null

  trybox events "$run_id" >"$TMP/vm-events.txt"
  grep -q command_finished "$TMP/vm-events.txt"

  trybox events "$run_id" --json >"$TMP/vm-events.json"
  jq empty "$TMP/vm-events.json"
  jq -e 'any(.[]; .type == "command_finished" and .phase == "command")' "$TMP/vm-events.json" >/dev/null

  trybox run --json -- sh -lc "printf 'json-ok\\n'" >"$TMP/vm-run-json.json"
  jq empty "$TMP/vm-run-json.json"
  jq -e '.exit_code == 0 and .sync != null and (.command_string | contains("json-ok")) and (.command_duration | length > 0)' "$TMP/vm-run-json.json" >/dev/null

  trybox history --limit 5 --json >"$TMP/vm-history.json"
  jq empty "$TMP/vm-history.json"

  trybox status --json >"$TMP/vm-status.json"
  jq empty "$TMP/vm-status.json"

  printf 'integration: opening Tart VNC endpoint\n' >&2
  trybox view --vnc | tee "$TMP/vnc-view.txt"
  grep -q '^display:   tart-vnc$' "$TMP/vnc-view.txt"
  grep -q '^client:    none$' "$TMP/vnc-view.txt"
  grep -q '^url:       vnc://' "$TMP/vnc-view.txt"
  grep -q '^open:      skipped$' "$TMP/vnc-view.txt"

  view_pause="${TRYBOX_VIEW_PAUSE_SECONDS:-60}"
  printf 'integration: leaving VNC endpoint running for %s seconds\n' "$view_pause" >&2
  sleep "$view_pause"

  printf 'integration: destroying VM\n' >&2
  trybox destroy
fi

if [[ "${TRYBOX_INTEGRATION_FIREFOX:-}" == "1" ]]; then
  printf 'TRYBOX_INTEGRATION_FIREFOX is no longer needed; ~/firefox is the default integration repo\n' >&2
fi

printf 'integration check passed\n'
