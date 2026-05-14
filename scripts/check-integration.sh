#!/usr/bin/env bash
set -Eeuo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
REAL_HOME="${HOME:-}"
TRYBOX_HOME="$TMP/home"
TRYBOX_BIN="$TMP/bin/trybox"
VM_WORKSPACE_ID=""

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  if [[ -n "$VM_WORKSPACE_ID" && "${TRYBOX_INTEGRATION_VM:-}" == "1" ]]; then
    HOME="$TRYBOX_HOME" "$TRYBOX_BIN" destroy "$VM_WORKSPACE_ID" --json >/dev/null 2>&1 || true
  fi
  chmod -R u+w "$TMP" 2>/dev/null || true
  rm -rf "$TMP"
}

trap 'printf "integration check failed near line %s\n" "$LINENO" >&2' ERR
trap cleanup EXIT

need_tool() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 not found in PATH"
}

json_assert() {
  local file="$1"
  local expr="$2"
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
  {
    printf '+ trybox'
    printf ' %q' "$@"
    printf '\n'
  } >&2
  HOME="$TRYBOX_HOME" "$TRYBOX_BIN" "$@"
}

run_json() {
  local name="$1"
  shift
  local out="$TMP/$name.json"
  run_trybox "$@" >"$out"
  python3 -m json.tool "$out" >/dev/null
  printf '%s\n' "$out"
}

create_fixture_repo() {
  local repo="$1"
  mkdir -p "$repo"
  git -C "$repo" init -q
  git -C "$repo" config user.email trybox-integration@example.invalid
  git -C "$repo" config user.name "Trybox Integration"

  printf 'ignored.log\n' >"$repo/.gitignore"
  printf 'excluded.txt\n' >"$repo/.tryboxignore"
  printf 'tracked\n' >"$repo/tracked.txt"
  printf 'ignored\n' >"$repo/ignored.log"
  printf 'excluded\n' >"$repo/excluded.txt"
  git -C "$repo" add .gitignore tracked.txt

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
  if ! tart list 2>/dev/null | grep -q 'trybox-macos15-arm64-image'; then
    printf 'TRYBOX_INTEGRATION_VM=1 set, but trybox-macos15-arm64-image is missing; skipping VM-backed checks\n' >&2
    return
  fi

  local fixture="$1"
  local workspace_json
  pushd "$fixture" >/dev/null
  workspace_json="$(run_json vm-workspace-use workspace use --target macos15-arm64 --json)"
  VM_WORKSPACE_ID="$(json_get "$workspace_json" "data['workspace']['id']")"

  run_json vm-doctor doctor --target macos15-arm64 --json >/dev/null
  run_json vm-up up --json >/dev/null
  run_json vm-sync sync --json >/dev/null

  local run_json_path
  local run_id
  run_json_path="$(run_json vm-run run --json -- sh -lc 'printf vm-ok')"
  run_id="$(json_get "$run_json_path" "data['id']")"
  run_trybox logs "$run_id" >"$TMP/vm-logs.txt"
  grep -q 'vm-ok' "$TMP/vm-logs.txt"
  run_json vm-events events "$run_id" --json >/dev/null
  run_json vm-history history --limit 5 --json >/dev/null
  run_json vm-stop stop --json >/dev/null
  run_json vm-destroy destroy "$VM_WORKSPACE_ID" --json >/dev/null
  VM_WORKSPACE_ID=""
  popd >/dev/null
}

need_tool go
need_tool git
need_tool python3

mkdir -p "$TRYBOX_HOME" "$(dirname "$TRYBOX_BIN")"
real_trybox_existed=0
if [[ -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  real_trybox_existed=1
fi

cd "$ROOT"
printf '+ go test ./...\n'
go test ./...
printf '+ go build -o %q ./cmd/trybox\n' "$TRYBOX_BIN"
go build -o "$TRYBOX_BIN" ./cmd/trybox

fixture="$TMP/fixture-repo"
create_fixture_repo "$fixture"

target_json="$(run_json target-list target list --json)"
json_assert "$target_json" "isinstance(data, list) and any(target.get('name') == 'macos15-arm64' for target in data)"

pushd "$fixture" >/dev/null

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
grep -q 'trybox workspace use' "$TMP/help-workspace.txt"

if run_trybox workspace nope >"$TMP/negative.out" 2>&1; then
  fail "unknown workspace subcommand unexpectedly succeeded"
fi
grep -q 'unknown workspace subcommand' "$TMP/negative.out"

popd >/dev/null

[[ -d "$TRYBOX_HOME/.trybox" ]] || fail "expected temporary Trybox state under isolated HOME"
if [[ "$real_trybox_existed" == "0" && -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  fail "integration check created state under the caller's real HOME"
fi

run_vm_mode "$fixture"

printf 'integration check passed\n'
