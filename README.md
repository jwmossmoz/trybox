# Trybox

Clean local VM workspaces for source debugging.

Trybox is a local counterpart to the "try it somewhere clean" workflow: sync a
dirty checkout into a fresh VM, run the command there, and keep durable logs.
The first backend is Tart on Apple Silicon macOS.

## What It Does

- Creates repo-bound workspace VMs from local target images.
- Syncs tracked files, VCS metadata, and nonignored local changes.
- Runs checkout-local commands in the guest.
- Opens a desktop through Tart's native window or Tart VNC.
- Stores run logs, events, and workspace state under `~/.trybox`.

## Quick Start

Prerequisites:

- Apple Silicon macOS host
- Tart installed
- SSH-ready Trybox macOS target image
- Source checkout on the host

```sh
go run ./cmd/trybox doctor
go run ./cmd/trybox target list
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
go run ./cmd/trybox workspace use --target macos15-arm64 --cpu 10 --memory-mb 24576 --disk-gb 100 ~/src/project
go run ./cmd/trybox up
go run ./cmd/trybox sync
go run ./cmd/trybox run -- env
```

Build a local binary when the command shape is stable enough for repeated use:

```sh
go build -o trybox ./cmd/trybox
./trybox status
```

## Development Checks

Run the local integration check before opening a PR:

```sh
scripts/check-integration.sh
```

The default check runs `go test ./...`, builds `trybox`, and exercises the
PR-safe CLI workflow against an isolated temporary `HOME` and disposable git
fixture. It does not require Tart or a local target image.

To include the VM-backed workflow on a host with Tart and a local target image:

```sh
TRYBOX_INTEGRATION_VM=1 scripts/check-integration.sh
```

## Common Flows

Read the full [CLI guide](docs/cli.md) for definitions of targets,
workspaces, workspace VMs, sync plans, runs, and every public flag.

Set or inspect the default workspace:

```sh
trybox workspace show
trybox workspace use --target macos15-arm64 ~/src/project
```

Plan and sync a dirty checkout:

```sh
trybox sync-plan
trybox sync
```

Run a command and inspect logs:

```sh
trybox run -- ./build-or-test-command test
trybox history
trybox events <run-id>
trybox logs <run-id>
```

Open the guest desktop:

```sh
trybox view                  # Tart native display
trybox view --vnc            # Tart VNC plus Apple's Screen Sharing client
trybox view --vnc --no-open  # Tart VNC endpoint only
```

## CLI Contract

- Human-readable output by default.
- `--json` for agent/script output on commands that return structured state.
- Diagnostics and command stderr go to stderr.
- Workspace defaults avoid repeating `--repo` on every command.
- `view --json` implies `--no-open`.
- `--no-open` means Trybox does not open a host GUI client.

## Target Images

`trybox up` expects a local target image that is already SSH-ready. A target
image is a reusable local base for a target such as `macos15-arm64`; a workspace
VM is the disposable repo-bound clone created from it.

Planned first-time setup:

```sh
trybox bootstrap --target macos15-arm64
```

`trybox target list` shows whether each local target image is present and, when
missing, the exact clone command bootstrap would run. You can still create the
local target image manually with Tart:

```sh
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
```

## Commands

```sh
trybox bootstrap [--target name] [--json]
trybox destroy [<workspace-id>] [--json]
trybox doctor [--target name] [--json]
trybox events <run-id> [--json]
trybox fetch --url URL --to guest-path [--target name] [--repo path] [--json]
trybox history [--limit n] [--json]
trybox info [--json]
trybox logs <run-id> [--follow|-f] [--from-end]
trybox reset [--target name] [--repo path] [--json]
trybox run [--target name] [--repo path] [--json] -- <command>
trybox shell [--target name] [--repo path] [-- <command>]
trybox snapshot save <name> [--target name] [--repo path] [--json]
trybox snapshot list [--target name] [--repo path] [--json]
trybox snapshot restore <name> [--display] [--target name] [--repo path] [--json]
trybox snapshot delete <name> [--target name] [--repo path] [--json]
trybox status [--target name] [--repo path] [--json]
trybox stop [--target name] [--repo path] [--json]
trybox sync [--target name] [--repo path] [--json]
trybox sync-plan [--repo path] [--limit n] [--json]
trybox task <task-id> [run|shell] [--root-url URL] [--target name] [--repo path] [--json]
trybox target list [--json]
trybox try <revision-or-url> [task <task-id> [run|shell]] [--root-url URL] [--target name] [--repo path] [--json]
trybox up [--target name] [--repo path] [--profile test|build] [--cpu n] [--memory-mb n] [--disk-gb n] [--json]
trybox view [--target name] [--repo path] [--vnc] [--no-open] [--reuse-client] [--restart-display] [--json]
trybox workspace list [--json]
trybox workspace show [--json]
trybox workspace unset [--json]
trybox workspace use [--target name] [--profile test|build] [--cpu n] [--memory-mb n] [--disk-gb n] [--json] [repo]
```

`trybox destroy` deletes only the selected workspace VM. Without a workspace
id, it selects the configured default workspace. It does not delete the host
checkout, run logs, or workspace metadata. Stale runtime state on the workspace
record (last known IP, sync fingerprint, last sync timestamp, last run log) is
cleared so the next `trybox up` starts fresh.

`trybox task` fetches a Taskcluster task definition from `--root-url` (or
`TASKCLUSTER_ROOT_URL`) and turns it into a local replay plan. The default mode
prints the resolved command, environment count, dependencies, artifacts, and
target mapping. Add `run` to sync the selected workspace and execute the task
command, or `shell` to open an SSH shell with the task environment exported.

`trybox try <revision-or-url>` checks whether the selected host checkout is at
the requested revision. With `task <task-id>`, it combines the source check with
the Taskcluster replay plan and refuses `run`/`shell` when the checkout revision
does not match.

`--profile test` selects a smaller VM shape for short test or harness work.
`--profile build` selects a larger source-build shape. Explicit `--cpu`,
`--memory-mb`, and `--disk-gb` values override the selected profile.

`trybox fetch --url URL --to path` downloads an artifact from inside the guest.
Relative destinations are resolved under the guest work path.

`trybox reset` deletes and recreates the selected workspace VM, then syncs the
checkout back into the clean guest.

## Guest Paths and Shell Expansion

The synced checkout lives at `/Users/admin/trybox` inside the guest. The host
shell expands `~` before `trybox run` sees the argv, so `~/trybox` resolves to
the host home, not the guest's. Use the absolute guest path, or wrap the
command in single quotes to defer expansion:

```sh
trybox run -- bash -c 'cd /Users/admin/trybox && ./mach --help'
trybox run -- bash -lc 'cd "$HOME/trybox" && ./mach --help'
```

## More Detail

- [CLI guide](docs/cli.md)
- [Architecture](docs/architecture.md)
- [Image model](docs/images.md)
- [Security model](docs/security.md)
- [Agent instructions](AGENTS.md)
- [Trybox agent skill](.agents/skills/trybox/SKILL.md)
