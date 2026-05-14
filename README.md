# Trybox

Run a dirty checkout in a clean local VM.

Trybox is a local counterpart to the "try it somewhere clean" workflow: it
syncs your current checkout into a local VM, runs the command there, streams
output, and keeps durable logs. The first backend is Tart on Apple Silicon
macOS.

## What It Does

- Creates and reuses a VM for the selected repo and target.
- Syncs tracked files, VCS metadata, and nonignored local changes before each run.
- Runs checkout-local commands in the guest.
- Opens the VM desktop through Tart's native window or Tart VNC.
- Stores run logs, events, and VM state under `~/.trybox`.

## Quick Start

Prerequisites:

- Apple Silicon macOS host
- Tart installed
- SSH-ready Trybox macOS target image
- Source checkout on the host

```sh
go run ./cmd/trybox doctor
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
go run ./cmd/trybox run --target macos15-arm64 -- env
go run ./cmd/trybox logs
go run ./cmd/trybox view --vnc
```

Build a local binary when the command shape is stable enough for repeated use:

```sh
go build -o trybox ./cmd/trybox
./trybox run -- ./build-or-test-command test
```

## Defaults

`trybox run -- <command>` is the main workflow. It picks defaults in this order:

- Target: `--target`, then `TRYBOX_TARGET`, then saved config, then `macos15-arm64`.
- Repo: `--repo`, then `TRYBOX_REPO`, then the current git checkout.
- VM resources: `--cpu`, `--memory-mb`, `--disk-gb`, then `TRYBOX_CPU`,
  `TRYBOX_MEMORY_MB`, `TRYBOX_DISK_GB`, then target defaults.

Passing `--target` to a VM-backed command records it as the next default target.

## Common Flows

Run a command and inspect the latest log:

```sh
trybox run -- ./build-or-test-command test
trybox logs
trybox history
trybox events <run-id>
```

Open the guest desktop:

```sh
trybox view                  # Tart native display
trybox view --vnc            # Tart VNC endpoint
trybox view --vnc --json     # Tart VNC endpoint as JSON
```

Inspect or remove the VM for the current repo and target:

```sh
trybox status
trybox destroy
```

## CLI Contract

- Human-readable output by default.
- `--json` for agent/script output on commands that return structured state.
- Diagnostics and command stderr go to stderr.
- `trybox run` sync status goes to stderr; command stdout streams to stdout.
- `trybox logs` prints the latest run log unless a run id is provided.
- `trybox view --vnc` starts Tart's VNC server and prints the localhost endpoint
  for your VNC client.

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

## Commands

```sh
trybox destroy [--target name] [--repo path] [--json]
trybox doctor [--target name] [--json]
trybox events <run-id> [--json]
trybox history [--limit n] [--json]
trybox logs [run-id]
trybox run [--target name] [--repo path] [--cpu n] [--memory-mb n] [--disk-gb n] [--json] -- <command>
trybox status [--target name] [--repo path] [--json]
trybox target list [--json]
trybox view [--target name] [--repo path] [--vnc] [--json]
```

`trybox destroy` deletes only the selected VM. It does not delete the host
checkout, run logs, or metadata. Runtime state on the VM record is cleared so
the next `trybox run` starts fresh.

## Guest Paths and Shell Expansion

The synced checkout lives at `/Users/admin/trybox` inside the guest. The host
shell expands `~` before `trybox run` sees the argv, so `~/trybox` resolves to
the host home, not the guest's. Use the absolute guest path, or wrap the command
in single quotes to defer expansion:

```sh
trybox run -- bash -c 'cd /Users/admin/trybox && ./mach --help'
trybox run -- bash -lc 'cd "$HOME/trybox" && ./mach --help'
```

## More Detail

- [Architecture](docs/architecture.md)
- [Image model](docs/images.md)
- [Security model](docs/security.md)
- [Agent instructions](AGENTS.md)
- [Trybox agent skill](.agents/skills/trybox/SKILL.md)
