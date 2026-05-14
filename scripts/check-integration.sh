#!/usr/bin/env bash
set -Eeuo pipefail
set -x

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
TMP="$(cd "$TMP" && pwd -P)"
REAL_HOME="${HOME:-}"
TRYBOX_HOME="$TMP/home"
TRYBOX="$TMP/bin/trybox"
FIXTURE="$TMP/fixture-repo"

cleanup() {
  set +e
  if [[ -x "$TRYBOX" && -d "${FIXTURE:-}" ]]; then
    HOME="$TRYBOX_HOME" "$TRYBOX" destroy --target macos15-arm64 --repo "$FIXTURE" --json >/dev/null 2>&1
  fi
  if [[ -x "$TRYBOX" && -n "${FIREFOX_REPO_USED:-}" ]]; then
    HOME="$TRYBOX_HOME" "$TRYBOX" destroy --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >/dev/null 2>&1
  fi
  chmod -R u+w "$TMP" 2>/dev/null || true
  rm -rf "$TMP"
}

trap 'printf "integration check failed near line %s\n" "$LINENO" >&2' ERR
trap cleanup EXIT

command -v go
command -v git
command -v python3

mkdir -p "$TRYBOX_HOME" "$(dirname "$TRYBOX")"
real_trybox_existed=0
if [[ -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  real_trybox_existed=1
fi

cd "$ROOT"
go test ./...
go build -o "$TRYBOX" ./cmd/trybox

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

HOME="$TRYBOX_HOME" "$TRYBOX" target list --json >"$TMP/targets.json"
python3 -m json.tool "$TMP/targets.json" >/dev/null
python3 - "$TMP/targets.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    targets = json.load(f)
assert any(target.get("name") == "macos15-arm64" for target in targets)
PY

HOME="$TRYBOX_HOME" "$TRYBOX" status --target macos15-arm64 --repo "$FIXTURE" --json >"$TMP/status.json"
python3 -m json.tool "$TMP/status.json" >/dev/null
python3 - "$TMP/status.json" "$FIXTURE" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    status = json.load(f)
assert status["vm"]["repo_root"] == sys.argv[2]
assert status["exists"] is False
assert status["running"] is False
PY

HOME="$TRYBOX_HOME" "$TRYBOX" help run >"$TMP/help-run.txt"
grep -q TRYBOX_TARGET "$TMP/help-run.txt"

if HOME="$TRYBOX_HOME" "$TRYBOX" workspace >"$TMP/negative.out" 2>&1; then
  printf 'removed workspace command unexpectedly succeeded\n' >&2
  exit 1
fi
grep -q 'unknown command "workspace"' "$TMP/negative.out"

[[ -d "$TRYBOX_HOME/.trybox" ]]
if [[ "$real_trybox_existed" == "0" && -n "$REAL_HOME" && -e "$REAL_HOME/.trybox" ]]; then
  printf 'integration check created state under the caller real HOME\n' >&2
  exit 1
fi

if [[ "${TRYBOX_INTEGRATION_VM:-}" == "1" ]]; then
  if command -v tart >/dev/null 2>&1 && tart list 2>/dev/null | grep -q 'trybox-macos15-arm64-image'; then
    HOME="$TRYBOX_HOME" "$TRYBOX" run --target macos15-arm64 --repo "$FIXTURE" --json -- sh -lc 'pwd && test -d .git && printf vm-ok' >"$TMP/vm-run.json"
    python3 -m json.tool "$TMP/vm-run.json" >/dev/null
    python3 - "$TMP/vm-run.json" >"$TMP/vm-run-id.txt" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    print(json.load(f)["id"])
PY
    HOME="$TRYBOX_HOME" "$TRYBOX" logs >"$TMP/vm-latest-log.txt"
    grep -q vm-ok "$TMP/vm-latest-log.txt"
    HOME="$TRYBOX_HOME" "$TRYBOX" events "$(cat "$TMP/vm-run-id.txt")" --json >"$TMP/vm-events.json"
    python3 -m json.tool "$TMP/vm-events.json" >/dev/null
    HOME="$TRYBOX_HOME" "$TRYBOX" history --limit 5 --json >"$TMP/vm-history.json"
    python3 -m json.tool "$TMP/vm-history.json" >/dev/null
    HOME="$TRYBOX_HOME" "$TRYBOX" status --target macos15-arm64 --repo "$FIXTURE" --json >"$TMP/vm-status.json"
    python3 -m json.tool "$TMP/vm-status.json" >/dev/null
    HOME="$TRYBOX_HOME" "$TRYBOX" view --target macos15-arm64 --repo "$FIXTURE" --vnc --json >"$TMP/vm-view.json"
    python3 -m json.tool "$TMP/vm-view.json" >/dev/null
    python3 - "$TMP/vm-view.json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as f:
    view = json.load(f)
assert view["display"] == "tart-vnc"
assert view["client"] == "none"
assert view["url"].startswith("vnc://")
PY
    HOME="$TRYBOX_HOME" "$TRYBOX" destroy --target macos15-arm64 --repo "$FIXTURE" --json >"$TMP/vm-destroy.json"
    python3 -m json.tool "$TMP/vm-destroy.json" >/dev/null
  else
    printf 'TRYBOX_INTEGRATION_VM=1 set, but tart or trybox-macos15-arm64-image is missing; skipping VM-backed checks\n' >&2
  fi
fi

if [[ "${TRYBOX_INTEGRATION_FIREFOX:-}" == "1" ]]; then
  command -v tart
  tart list 2>/dev/null | grep -q 'trybox-macos15-arm64-image'
  FIREFOX_REPO_USED="${FIREFOX_REPO:-$HOME/firefox}"
  [[ -d "$FIREFOX_REPO_USED" ]]
  [[ -x "$FIREFOX_REPO_USED/mach" ]]
  mach_command="${TRYBOX_FIREFOX_MACH_COMMAND:-if command -v python3.11 >/dev/null 2>&1; then py=python3.11; else py=python3; fi; \"\$py\" ./mach python -c 'print(\"mach-python-ok\")'}"
  HOME="$TRYBOX_HOME" "$TRYBOX" run --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json -- sh -lc "$mach_command" >"$TMP/firefox-run.json"
  python3 -m json.tool "$TMP/firefox-run.json" >/dev/null
  HOME="$TRYBOX_HOME" "$TRYBOX" logs >"$TMP/firefox-log.txt"
  grep -q mach-python-ok "$TMP/firefox-log.txt"
  HOME="$TRYBOX_HOME" "$TRYBOX" status --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >"$TMP/firefox-status.json"
  python3 -m json.tool "$TMP/firefox-status.json" >/dev/null
  HOME="$TRYBOX_HOME" "$TRYBOX" destroy --target macos15-arm64 --repo "$FIREFOX_REPO_USED" --json >"$TMP/firefox-destroy.json"
  python3 -m json.tool "$TMP/firefox-destroy.json" >/dev/null
fi

printf 'integration check passed\n'
