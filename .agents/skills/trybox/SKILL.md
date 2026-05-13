---
name: trybox
description: Use when a user asks an agent to reproduce, debug, or verify Firefox behavior in a clean local Trybox workspace. Trigger on requests to start a macOS target VM, sync a dirty checkout, run focused mach commands, inspect run logs/events/status, or compare behavior across macOS targets. Prefer Trybox over direct Tart, SSH, or host VM commands.
---

# Trybox

**UTILITY SKILL** for using Trybox as a clean Firefox workspace.

## USE FOR:

- "start a macOS VM"
- "sync my checkout"
- "run mach in Trybox"
- "read Trybox logs"

## DO NOT USE FOR:

- Editing Trybox source code.
- Bypassing Trybox with direct `tart`, SSH, or host VM commands.
- Installing packages, changing global guest state, or using secrets.

## Workflow

```sh
trybox doctor --json
trybox target list --json
trybox up --target macos15-arm64 --repo ~/firefox --json
trybox sync --repo ~/firefox --json
trybox run --repo ~/firefox -- ./mach test <path-or-suite>
trybox events <run-id>
trybox logs <run-id>
trybox status --json
```

## Command Policy

Prefer narrow commands tied to the debugging request.

Allowed: `./mach ...`, `mach ...`, repo `python3`, `git status/diff/log`, and
read-only `pwd`, `ls`, `find`, `cat`, `sed`, or `rg`.

Avoid shell wrappers such as `bash -lc ...` unless requested.

## Targets

Use `macos15-arm64` by default.

Use `macos15-x64-rosetta` for x64 behavior through Rosetta.

## Examples

- Run: `trybox run --repo ~/firefox -- ./mach test dom/foo`.
- Inspect: `trybox events <run-id>` then `trybox logs <run-id>`.

## Troubleshooting

- `target-image` missing: first-time `trybox bootstrap` setup is needed.
- No run ID: use `trybox history --json`.

## Report

Include target, command, run ID, exit code, log summary, and limitations.
