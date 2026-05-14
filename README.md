# Trybox

Run a dirty checkout in a clean local VM.

```sh
trybox run -- ./build-or-test-command test
```

Trybox is the local version of "try this somewhere clean." It creates or reuses
a VM for your current source checkout, syncs the dirty checkout into the guest,
runs the command you pass, streams output, and keeps durable logs/events for
humans and agents. The first backend is Tart on Apple Silicon macOS.

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

## Quick Start

Prerequisites:

- Apple Silicon macOS host
- Tart installed
- Source checkout on the host
- SSH-ready Trybox target image

From this repository, build or install Trybox:

```sh
go install ./cmd/trybox
```

Create the default target image until `trybox bootstrap` exists:

```sh
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
```

Run from a source checkout:

```sh
cd ~/src/project
trybox doctor
trybox run -- ./build-or-test-command test
trybox logs
trybox view
trybox view --vnc
```

The normal command is intentionally short. `trybox run -- <command>` selects the
repo, selects the target, creates or starts the VM, syncs the checkout, runs the
command in `/Users/admin/trybox`, streams output, and records logs/events.

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

## What To Tell An Agent

When handing Trybox work to an agent, paste a single quoted prompt like this and
replace `<checkout>` and `<command>`:

```sh
'Set up Trybox on this Apple Silicon macOS host and use it for <checkout>: require or install prerequisites (`brew install go cirruslabs/cli/tart` if missing); install Trybox (`go install github.com/jwmossmoz/trybox/cmd/trybox@main`, or `go install ./cmd/trybox` when already in the Trybox source repo); set `TRYBOX_REPO=<checkout>` and `TRYBOX_TARGET=${TRYBOX_TARGET:-macos15-arm64}`; run `trybox doctor --json`; if the target image is missing, run `tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image` and rerun doctor; run `trybox run -- <command>` and stream output; after completion run `trybox logs`, get the latest run id with `trybox history --limit 1 --json`, and inspect `trybox events <run-id> --json`; use `trybox view` for Tart native desktop and `trybox view --vnc` only when I ask for a Tart VNC endpoint.'
```

## Desktop Access

Open the VM desktop with Tart's native window:

```sh
trybox view
```

Start Tart's VNC endpoint and print the URL for your own client:

```sh
trybox view --vnc
trybox view --vnc --json
```

`trybox view --vnc` does not launch Apple Screen Sharing. It starts Tart's VNC
mode, prints the localhost `vnc://` endpoint, and leaves client choice to you.

## Defaults

Trybox chooses defaults so the normal command stays short:

| Setting | Resolution order |
| --- | --- |
| Target | `--target`, then `TRYBOX_TARGET`, then saved config, then `macos15-arm64`. |
| Repo | `--repo`, then `TRYBOX_REPO`, then the current git checkout. |
| Resources | `--cpu`, `--memory-mb`, `--disk-gb`, then `TRYBOX_CPU`, `TRYBOX_MEMORY_MB`, `TRYBOX_DISK_GB`, then target defaults. |

Passing `--target` to a VM-backed command records it as the next default target.

## Common Flows

Run a command and inspect the latest log:

```sh
trybox run -- ./build-or-test-command test
trybox logs
trybox history
trybox events <run-id>
```

Inspect or remove the VM for the current repo and target:

```sh
trybox status
trybox destroy
```

Use a different checkout or target without changing directories:

```sh
TRYBOX_REPO=~/src/project TRYBOX_TARGET=macos15-arm64 trybox run -- ./build-or-test-command
```

## Target Images

`trybox run`, `trybox status`, `trybox view`, and `trybox destroy` operate on the
VM for the selected repo and target. The first run expects a local target image
that is already SSH-ready.

Planned first-time setup:

```sh
trybox bootstrap --target macos15-arm64
```

Until `bootstrap` exists, create the local target image manually with Tart:

```sh
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
```

List built-in targets and image status:

```sh
trybox target list
trybox target list --json
```

## Commands

```sh
trybox destroy [--target name] [--repo path] [--json]
trybox doctor [--target name] [--json]
trybox events <run-id> [--json]
trybox history [--limit n] [--json]
trybox logs [run-id] [--json]
trybox run [--target name] [--repo path] [--cpu n] [--memory-mb n] [--disk-gb n] [--json] -- <command>
trybox status [--target name] [--repo path] [--json]
trybox target list [--json]
trybox view [--target name] [--repo path] [--vnc] [--json]
```

`trybox destroy` deletes only the selected VM. It does not delete the host
checkout, run logs, or metadata. Runtime state on the VM record is cleared so
the next `trybox run` starts fresh.

## Guest Paths And Shell Expansion

The synced checkout lives at `/Users/admin/trybox` inside the guest. The host
shell expands `~` before `trybox run` sees the argv, so `~/trybox` resolves to
the host home, not the guest home. Use the absolute guest path, or wrap the
command in single quotes to defer expansion:

```sh
trybox run -- bash -c 'cd /Users/admin/trybox && ./build-or-test-command'
trybox run -- bash -lc 'cd "$HOME/trybox" && ./build-or-test-command'
```

## Development Checks

The repository includes an end-to-end check that builds the local CLI, runs real
Trybox commands, validates JSON/log/event output, opens the Tart native window,
starts Tart VNC, and destroys the test VM:

```sh
./ci/check-integration.sh
```

It defaults to `~/firefox` today because that is the large local checkout used
to exercise dirty-source sync behavior. Override it with `TRYBOX_REPO` when you
want to run the check against another source checkout.

## More Detail

- [CLI guide](docs/cli.md)
- [Architecture](docs/architecture.md)
- [Image model](docs/images.md)
- [Security model](docs/security.md)
- [Agent instructions](AGENTS.md)
- [Trybox agent skill](.agents/skills/trybox/SKILL.md)
