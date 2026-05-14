# Trybox Overview

Trybox is the local version of "try this somewhere clean." It creates or reuses
a VM for your current source checkout, syncs the dirty checkout into the guest,
runs the command you pass, streams output, and keeps durable logs/events for
humans and agents.

## What It Does

- Creates one VM for each selected repo and target.
- Syncs tracked files, VCS metadata, and nonignored local changes before each
  run.
- Runs checkout-local commands from the guest checkout.
- Streams command output while also recording durable logs and events.
- Opens the VM desktop through Tart's native window or Tart's VNC endpoint.
- Stores VM metadata, run logs, events, backend logs, and SSH keys under
  `~/.trybox`.

Trybox does not guess what command you meant to run. If you want to launch an
app, pass the app launch command. If your command needs build artifacts, build
them in the guest or make sure they are included in the synced checkout.

## What Happens On `run`

1. Trybox resolves the target and source checkout.
2. It creates or reuses the repo-bound VM for that target.
3. It starts the VM headless if it is not already running.
4. It builds a manifest from tracked files, VCS metadata, and nonignored local
   files.
5. It syncs that manifest into the guest checkout and removes stale files.
6. It runs the command from `/Users/admin/trybox`.
7. It records stdout, stderr, combined logs, run metadata, and events under
   `~/.trybox/runs`.

## Output For Humans And Agents

Human output is the default. `trybox run` prints VM, sync, command, summary, and
log pointers while keeping guest stdout on stdout:

```text
run context:
  run=run_... logs=trybox logs run_... events=trybox events run_...
  target=macos15-arm64 vm=trybox-ws-... repo=/path/to/project
  workdir=/Users/admin/trybox
vm:        ensuring trybox-ws-...
sync:      preparing checkout
command:   './build-or-test-command' 'test'
summary:   sync=... command=... total=... exit=0
logs:      trybox logs run_...
```

Use `--json` when a script or agent needs structured output:

```sh
trybox status --json
trybox run --json -- ./build-or-test-command test
trybox logs run_... --json
trybox events run_... --json
```

For human runs, Trybox phase/status lines and command stderr go to stderr, while
guest stdout stays on stdout. With `--json`, stdout is reserved for JSON and
command output is available through `trybox logs`. Automation should read JSON
from stdout and treat stderr as diagnostics.

## Paths And Shell Expansion

The synced checkout lives at `/Users/admin/trybox` inside the guest. The host
shell expands `~` before `trybox run` sees the argv, so `~/trybox` resolves to
the host home, not the guest home. Use the absolute guest path, or wrap the
command in single quotes to defer expansion:

```sh
trybox run -- bash -c 'cd /Users/admin/trybox && ./build-or-test-command'
trybox run -- bash -lc 'cd "$HOME/trybox" && ./build-or-test-command'
```

## Development Check

The repository includes an end-to-end check that builds the local CLI, runs real
Trybox commands, validates JSON/log/event output, opens the Tart native window,
starts Tart VNC, and destroys the test VM:

```sh
./ci/check-integration.sh
```

Override the source checkout with `TRYBOX_REPO` when you want to run the check
against a specific repository.

For the heavier Firefox OS integration path, use:

```sh
./ci/check-firefox-os-integration.sh
```

That script runs the Firefox build plus the `os_integration` Marionette,
Mochitest, and xpcshell suites through Trybox. It defaults to
`TRYBOX_REPO`, `FIREFOX_REPO`, or `~/firefox`, and `TRYBOX_TARGET` or
`macos15-arm64`. If `trybox` is not already on `PATH`, it builds a temporary
local binary from this repository before running the check.
