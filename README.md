# Trybox

Clean local VM workspaces for source debugging.

Trybox is a local counterpart to Mozilla's "try" workflow: sync a dirty
checkout into a clean VM, run the command there, and keep durable logs. The
first backend is Tart on Apple Silicon macOS.

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
go run ./cmd/trybox workspace use --target macos15-arm64 --cpu 10 --memory-mb 24576 --disk-gb 100 ~/mozilla-unified
go run ./cmd/trybox up
go run ./cmd/trybox sync
go run ./cmd/trybox run -- ./mach --version
```

Build a local binary when the command shape is stable enough for repeated use:

```sh
go build -o trybox ./cmd/trybox
./trybox status
```

## Common Flows

Set or inspect the default workspace:

```sh
trybox workspace show
trybox workspace use --target macos15-arm64 ~/mozilla-unified
```

Plan and sync a dirty checkout:

```sh
trybox sync-plan
trybox sync
```

Run a command and inspect logs:

```sh
trybox run -- ./mach test browser/components/example
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
- `--no-open` means no host GUI client should remain open.

## Target Images

`trybox up` expects a local target image that is already SSH-ready. A target
image is a reusable local base for a target such as `macos15-arm64`; a workspace
VM is the disposable repo-bound clone created from it.

Planned first-time setup:

```sh
trybox bootstrap --target macos15-arm64
```

## Commands

```sh
trybox doctor [--json]
trybox target list [--json]
trybox workspace use [--target name] [--cpu n] [--memory-mb n] [--disk-gb n] [repo]
trybox workspace show [--json]
trybox workspace clear
trybox up [--target name] [--repo path] [--cpu n] [--memory-mb n] [--disk-gb n]
trybox sync [--target name] [--repo path] [--json]
trybox sync-plan [--repo path] [--limit n] [--json]
trybox run [--target name] [--repo path] [--json] -- <command>
trybox status [--target name] [--repo path] [--json]
trybox view [--target name] [--repo path] [--vnc] [--no-open] [--reuse-client] [--json]
trybox history [--limit n] [--json]
trybox logs <run-id>
trybox events <run-id> [--json]
trybox stop [--target name] [--repo path]
trybox destroy [--target name] [--repo path]
```

## More Detail

- [Architecture](docs/architecture.md)
- [Image model](docs/images.md)
- [Security model](docs/security.md)
- [Agent instructions](AGENTS.md)
- [Trybox agent skill](.agents/skills/trybox/SKILL.md)
