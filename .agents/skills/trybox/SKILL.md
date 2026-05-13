---
name: trybox
description: >
  Use when operating Trybox VM workspaces for source debugging: macOS VM
  startup, dirty checkout sync, Tart desktop/VNC, guest runs, and logs. DO NOT
  USE FOR editing Trybox itself; follow repo AGENTS.md.
---

# Trybox

**UTILITY SKILL** for using Trybox as a clean local VM workspace.

## USE FOR:

- Workspace start/status.
- Dirty checkout sync.
- Guest command runs.
- Desktop or VNC access.
- Logs and events.

## DO NOT USE FOR:

- Editing Trybox source.
- Direct `tart`/SSH unless debugging Trybox itself.
- Guest-global changes, secrets, or target image mutation.

## Workflow

```sh
trybox doctor --json
trybox target list --json
trybox workspace show --json
trybox workspace use --target <target> --cpu <n> --memory-mb <mib> --disk-gb <gib> <checkout>
trybox up --json
trybox sync --json
trybox run -- <command>
```

Use the configured workspace when correct. Set one when missing, wrong, or when
VM specs need changing. Choose targets from the user request or `target list`.
If `trybox up` reports a missing local target image before `trybox bootstrap`
exists, tell the user to create it manually, for example:

```sh
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
```

## Desktop

- `trybox view`: auto-login plus Tart native display.
- `trybox view --vnc`: Tart VNC plus Apple's Screen Sharing.
- `trybox view --vnc --no-open --json`: Tart VNC details only.
- `trybox view --restart-display`: explicitly restart a running VM to switch display mode.

## Examples

```sh
trybox run -- env
trybox view
trybox run -- <small visible test command>
```

## Troubleshooting

- Sync includes `.git`/`.hg`; repo tooling may need VCS metadata.
- Large syncs are expected; do not drop metadata just for speed.
- Guest workspace path is `/Users/admin/trybox`. The host shell expands `~`
  before `trybox run` sees argv, so `~/trybox` becomes the host home. Use
  the absolute guest path, or wrap the command in single quotes:
  `trybox run -- bash -c 'cd /Users/admin/trybox && ./mach --help'`.
- On failure, inspect `trybox events <run-id> --json` and `trybox logs
  <run-id>` before retrying.
- Early images may use `admin` / `admin`; `trybox view` automates auto-login.

## Report

Report target, workspace ID, VM specs when relevant, command, run ID, exit code,
log summary, and native Tart vs VNC mode.
