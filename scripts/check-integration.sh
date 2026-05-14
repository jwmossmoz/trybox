#!/usr/bin/env bash
set -Eeuo pipefail
exec 3>&2

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
REAL_HOME="${HOME:-}"
TRYBOX_HOME="$TMP/home"
TRYBOX_BIN="$TMP/bin/trybox"
VM_WORKSPACE_IDS=()

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

log_command() {
  local arg
  printf '+' >&3
  for arg in "$@"; do
    printf ' %q' "$arg" >&3
  done
  printf '\n' >&3
}

log_shell() {
  printf '+ %s\n' "$*" >&3
}

run_host() {
  log_command "$@"
  "$@"
}

pushd_logged() {
  log_command cd "$1"
  pushd "$1" >/dev/null
}

popd_logged() {
  log_shell "cd -"
  popd >/dev/null
}

cleanup() {
  local workspace_id
  if [[ ${#VM_WORKSPACE_IDS[@]} -gt 0 ]]; then
    for workspace_id in "${VM_WORKSPACE_IDS[@]}"; do
      log_command env "HOME=$TRYBOX_HOME" "$TRYBOX_BIN" destroy "$workspace_id" --json
      HOME="$TRYBOX_HOME" "$TRYBOX_BIN" destroy "$workspace_id" --json >/dev/null 2>&1 || true
    done
  fi
  log_command chmod -R u+w "$TMP"
  chmod -R u+w "$TMP" 2>/dev/null || true
  log_command rm -rf "$TMP"
  rm -rf "$TMP"
}

trap 'printf "integration check failed near line %s\n" "$LINENO" >&2' ERR
trap cleanup EXIT

need_tool() {
  log_command command -v "$1"
  command -v "$1" >/dev/null 2>&1 || fail "$1 not found in PATH"
}

json_assert() {
  local file="$1"
  local expr="$2"
  log_command python3 - "$file" "$expr"
  python3 - "$file" "$expr" <<'PY'
import json
import sys

path, expr = sys.argv[1], sys.argv[2]
with open(path, encoding="utf-8") as f:
    data = json.load(f)
if not eval(expr, {}, {"data": data}):
    raise SystemExit(f"JSON assertion failed for {path}: {expr}")
PY
}

json_get() {
  local file="$1"
  local expr="$2"
  log_command python3 - "$file" "$expr"
  python3 - "$file" "$expr" <<'PY'
import json
import sys

path, expr = sys.argv[1], sys.argv[2]
with open(path, encoding="utf-8") as f:
    data = json.load(f)
value = eval(expr, {}, {"data": data})
print(value)
PY
}

run_trybox() {
  log_command env "HOME=$TRYBOX_HOME" "$TRYBOX_BIN" "$@"
  HOME="$TRYBOX_HOME" "$TRYBOX_BIN" "$@"
}

run_json() {
  local name="$1"
  shift
  local out="$TMP/$name.json"
  run_trybox "$@" >"$out"
  run_host python3 -m json.tool "$out" >/dev/null
  printf '%s\n' "$out"
}

require_vm_prereqs() {
  log_command command -v tart
  command -v tart >/dev/null 2>&1 || fail "tart not found in PATH"
  log_shell "tart list | grep -q trybox-macos15-arm64-image"
  tart list 2>/dev/null | grep -q 'trybox-macos15-arm64-image' || fail "trybox-macos15-arm64-image is missing; run: trybox bootstrap --target macos15-arm64"
}

create_fixture_repo() {
  local repo="$1"
  run_host mkdir -p "$repo"
  run_host git -C "$repo" init -q
  run_host git -C "$repo" config user.email trybox-integration@example.invalid
  run_host git -C "$repo" config user.name "Trybox Integration"

  log_shell "write fixture files under $repo"
  printf 'ignored.log\n' >"$repo/.gitignore"
  printf 'excluded.txt\n' >"$repo/.tryboxignore"
  printf 'tracked\n' >"$repo/tracked.txt"
  printf 'ignored\n' >"$repo/ignored.log"
  printf 'excluded\n' >"$repo/excluded.txt"
  run_host git -C "$repo" add .gitignore tracked.txt

  log_shell "modify fixture files under $repo"
  printf 'changed\n' >"$repo/tracked.txt"
  printf 'untracked\n' >"$repo/untracked.txt"
}

run_vm_mode() {
  if [[ "${TRYBOX_INTEGRATION_VM:-}" != "1" ]]; then
    return
  fi
  if ! command -v tart >/dev/null 2>&1; then
    printf 'TRYBOX_INTEGRATION_VM=1 set, but tart is missing; skipping VM-backed checks\n' >&2
    return
  fi
  log_command command -v tart
  log_shell "tart list | grep -q trybox-macos15-arm64-image"
  if ! tart list 2>/dev/null | grep -q 'trybox-macos15-arm64-image'; then
    printf 'TRYBOX_INTEGRATION_VM=1 set, but trybox-macos15-arm64-image is missing; skipping VM-backed checks\n' >&2
    return
  fi

  local fixture="$1"
  local workspace_json
  local workspace_id
  pushd_logged "$fixture"
  workspace_json="$(run_json vm-workspace-use workspace use --target macos15-arm64 --json)"
  workspace_id="$(json_get "$workspace_json" "data['workspace']['id']")"
  VM_WORKSPACE_IDS+=("$workspace_id")

  run_json vm-doctor doctor --target macos15-arm64 --json >/dev/null
  run_json vm-up up --json >/dev/null
  run_json vm-status-running status --json >/dev/null
  run_json vm-sync sync --json >/dev/null
  run_json vm-fetch fetch --url file:///etc/hosts --to .trybox/etc-hosts --json >/dev/null

  local run_json_path
  local run_id
  run_json_path="$(run_json vm-run run --json -- sh -lc 'printf vm-ok')"
  run_id="$(json_get "$run_json_path" "data['id']")"
  run_trybox logs "$run_id" >"$TMP/vm-logs.txt"
  run_host grep -q 'vm-ok' "$TMP/vm-logs.txt"
  run_trybox logs "$run_id" --follow >"$TMP/vm-logs-follow.txt"
  run_host grep -q 'vm-ok' "$TMP/vm-logs-follow.txt"
  run_json vm-events events "$run_id" --json >/dev/null
  run_json vm-history history --limit 5 --json >/dev/null
  run_json vm-snapshot-save snapshot save fixture-smoke --json >/dev/null
  run_json vm-snapshot-list snapshot list --json >/dev/null
  run_json vm-snapshot-restore snapshot restore fixture-smoke --json >/dev/null
  run_json vm-snapshot-delete snapshot delete fixture-smoke --json >/dev/null
  run_json vm-stop stop --json >/dev/null
  run_json vm-destroy destroy "$workspace_id" --json >/dev/null
  VM_WORKSPACE_IDS=()
  popd_logged
}

run_firefox_mode() {
  if [[ "${TRYBOX_INTEGRATION_FIREFOX:-}" != "1" ]]; then
    return
  fi
  require_vm_prereqs

  local firefox_repo="${FIREFOX_REPO:-$HOME/firefox}"
  [[ -d "$firefox_repo" ]] || fail "Firefox repo not found: $firefox_repo"
  [[ -x "$firefox_repo/mach" ]] || fail "Firefox mach not found or not executable: $firefox_repo/mach"

  local workspace_json
  local workspace_id
  local run_json_path
  local run_id
  local mach_command
  mach_command="${TRYBOX_FIREFOX_MACH_COMMAND:-if command -v python3.11 >/dev/null 2>&1; then py=python3.11; else py=python3; fi; \"\$py\" ./mach python -c 'print(\"mach-python-ok\")'}"

  pushd_logged "$firefox_repo"
  workspace_json="$(run_json firefox-workspace-use workspace use --target macos15-arm64 --profile build --json)"
  workspace_id="$(json_get "$workspace_json" "data['workspace']['id']")"
  VM_WORKSPACE_IDS+=("$workspace_id")

  run_json firefox-info info --json >/dev/null
  run_json firefox-doctor doctor --target macos15-arm64 --json >/dev/null
  run_json firefox-up up --json >/dev/null
  run_json firefox-status status --json >/dev/null
  run_json firefox-sync sync --json >/dev/null

  run_json_path="$(run_json firefox-mach-run run --json -- sh -lc "$mach_command")"
  run_id="$(json_get "$run_json_path" "data['id']")"
  run_trybox logs "$run_id" --follow >"$TMP/firefox-mach-logs.txt"
  run_host grep -q 'mach-python-ok' "$TMP/firefox-mach-logs.txt"
  run_json firefox-events events "$run_id" --json >/dev/null
  run_json firefox-history history --limit 10 --json >/dev/null

  run_json firefox-snapshot-save snapshot save firefox-mach-smoke --json >/dev/null
  run_json firefox-snapshot-list snapshot list --json >/dev/null
  run_json firefox-snapshot-restore snapshot restore firefox-mach-smoke --json >/dev/null
  run_json firefox-snapshot-delete snapshot delete firefox-mach-smoke --json >/dev/null

  run_json firefox-stop stop --json >/dev/null
  run_json firefox-destroy destroy "$workspace_id" --json >/dev/null
  VM_WORKSPACE_IDS=()
  popd_logged
}

need_tool go
need_tool git
need_tool python3

run_host mkdir -p "$TRYBOX_HOME" "$(dirname "$TRYBOX_BIN")"
real_trybox_existed=0
if [[ -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  real_trybox_existed=1
fi

log_command cd "$ROOT"
cd "$ROOT"
run_host go test ./...
run_host go build -o "$TRYBOX_BIN" ./cmd/trybox

fixture="$TMP/fixture-repo"
create_fixture_repo "$fixture"

target_json="$(run_json target-list target list --json)"
json_assert "$target_json" "isinstance(data, list) and any(target.get('name') == 'macos15-arm64' for target in data)"

pushd_logged "$fixture"

workspace_json="$(run_json workspace-use workspace use --target macos15-arm64 --json)"
json_assert "$workspace_json" "data['default_target'] == 'macos15-arm64'"
json_assert "$workspace_json" "data['workspace']['repo_root']"

show_json="$(run_json workspace-show workspace show --json)"
json_assert "$show_json" "data['workspace']['target'] == 'macos15-arm64'"

list_json="$(run_json workspace-list workspace list --json)"
json_assert "$list_json" "len(data['workspaces']) == 1 and data['workspaces'][0]['is_default']"

status_json="$(run_json status status --json)"
json_assert "$status_json" "data['exists'] is False and data['running'] is False"

unset_json="$(run_json workspace-unset workspace unset --json)"
json_assert "$unset_json" "data['default_workspace_id'] == ''"

run_trybox help workspace >"$TMP/help-workspace.txt"
run_host grep -q 'trybox workspace use' "$TMP/help-workspace.txt"

if run_trybox workspace nope >"$TMP/negative.out" 2>&1; then
  fail "unknown workspace subcommand unexpectedly succeeded"
fi
run_host grep -q 'unknown workspace subcommand' "$TMP/negative.out"

popd_logged

[[ -d "$TRYBOX_HOME/.trybox" ]] || fail "expected temporary Trybox state under isolated HOME"
if [[ "$real_trybox_existed" == "0" && -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  fail "integration check created state under the caller's real HOME"
fi

run_vm_mode "$fixture"
run_firefox_mode

printf 'integration check passed\n'
