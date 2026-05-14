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

## Prerequisites

Before using Trybox, verify or ask the user for:

- A source checkout path. If the agent is not already inside that checkout, set
  `TRYBOX_REPO=<checkout>`.
- A target. Use `TRYBOX_TARGET` or default to `macos15-arm64` when the user does
  not specify one.
- A `trybox` binary on `PATH`. When working from a Trybox source checkout, build
  or install it with:

  ```sh
  go install ./cmd/trybox
  ```

- Tart installed on an Apple Silicon macOS host.
- A local SSH-ready target image. Until `trybox bootstrap` exists, create the
  default image with:

  ```sh
  tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
  ```

## What To Tell An Agent

Give the agent the checkout, target, and command:

```text
Use Trybox to run this checkout in a clean local VM.
Set TRYBOX_REPO=<checkout> and TRYBOX_TARGET=macos15-arm64 unless already configured.
Run trybox doctor --json, then trybox run -- <command>.
Stream the run output, then inspect trybox logs and trybox events <run-id> --json.
Use trybox view for Tart's native window and trybox view --vnc only when I ask for a VNC endpoint.
```

## Usage

```sh
trybox doctor --json
trybox target list --json
trybox run --target <target> --repo <checkout> --cpu <n> --memory-mb <mib> --disk-gb <gib> -- <command>
trybox logs
trybox history --limit 1 --json
trybox events <run-id> --json
```

Default repo is `TRYBOX_REPO` or the current git checkout. Default target is
`TRYBOX_TARGET`, saved config, or `macos15-arm64`. Choose targets from the user
request or `target list`.

## Desktop

- `trybox view`: auto-login plus Tart native display.
- `trybox view --vnc`: Tart VNC endpoint for the user's VNC client.
- `trybox view --vnc --json`: Tart VNC details as JSON.

## Examples

```sh
trybox run -- env
trybox logs
trybox view
trybox view --vnc
```

## Gotchas

- Trybox runs exactly the command passed after `--`; it does not infer that an
  app, browser, or test UI should be launched.
- Sync includes `.git`/`.hg`; repo tooling may need VCS metadata.
- Current sync follows tracked files, repository metadata, and nonignored local
  files. Ignored object directories and build artifacts are not assumed to exist
  in the guest.
- Large syncs are expected; do not drop metadata just for speed.
- Guest checkout path is `/Users/admin/trybox`. The host shell expands `~`
  before `trybox run` sees argv, so `~/trybox` becomes the host home. Use
  the absolute guest path, or wrap the command in single quotes:
  `trybox run -- bash -c 'cd /Users/admin/trybox && ./mach --help'`.
- On failure, inspect `trybox history`, `trybox events <run-id> --json`, and
  `trybox logs` before retrying.
- Early images may use `admin` / `admin`; `trybox view` automates auto-login.

## Reporting

Report target, VM name/specs when relevant, command, run ID, exit code, log
summary, and native Tart vs VNC mode.
