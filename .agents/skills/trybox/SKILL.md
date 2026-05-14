---
name: trybox
description: >
  Use when operating Trybox for source debugging: macOS VM startup, dirty
  checkout sync, Tart desktop/VNC, guest runs, and logs. DO NOT USE FOR editing
  Trybox itself; follow repo AGENTS.md.
---

# Trybox

**UTILITY SKILL** for using Trybox as a clean local VM runner.

## USE FOR:

- VM status.
- Dirty checkout sync through `trybox run`.
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
trybox run --target <target> --repo <checkout> --cpu <n> --memory-mb <mib> --disk-gb <gib> -- <command>
trybox logs
```

Default repo is `TRYBOX_REPO` or the current git checkout. Default target is
`TRYBOX_TARGET`, saved config, or `macos15-arm64`. Choose targets from the user
request or `target list`. If `trybox run` reports a missing local target image before `trybox bootstrap`
exists, tell the user to create it manually, for example:

```sh
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
```

## Desktop

- `trybox view`: auto-login plus Tart native display.
- `trybox view --vnc`: Tart VNC endpoint for the user's VNC client.
- `trybox view --vnc --json`: Tart VNC details as JSON.

## Examples

```sh
trybox run -- env
trybox view
trybox run -- <small visible test command>
```

## Troubleshooting

- Sync includes `.git`/`.hg`; repo tooling may need VCS metadata.
- Large syncs are expected; do not drop metadata just for speed.
- Guest checkout path is `/Users/admin/trybox`. The host shell expands `~`
  before `trybox run` sees argv, so `~/trybox` becomes the host home. Use
  the absolute guest path, or wrap the command in single quotes:
  `trybox run -- bash -c 'cd /Users/admin/trybox && ./mach --help'`.
- On failure, inspect `trybox history`, `trybox events <run-id> --json`, and
  `trybox logs` before retrying.
- Early images may use `admin` / `admin`; `trybox view` automates auto-login.

## Report

Report target, VM name/specs when relevant, command, run ID, exit code, log
summary, and native Tart vs VNC mode.
