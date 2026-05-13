# Trybox

Trybox creates clean local debugging workspaces for Firefox development.

The first backend is Tart on Apple Silicon macOS. It starts a clean macOS 15
arm64 VM, syncs the local Firefox checkout, runs commands inside the guest, and
stores durable logs for humans and agents. It intentionally does not run
production provisioning, inject CI credentials, or register with CI.

## Status

First cut. This is a private working repo while the macOS workflow is being
debugged.

## Quick Start

Prerequisites:

- Apple Silicon macOS host
- Tart installed
- A Trybox macOS target image with SSH enabled
- A Firefox checkout, defaulting to `~/firefox`

```sh
go run ./cmd/trybox doctor
go run ./cmd/trybox target list
go run ./cmd/trybox up --repo ~/firefox
go run ./cmd/trybox sync --repo ~/firefox
go run ./cmd/trybox run --repo ~/firefox -- ./mach --version
go run ./cmd/trybox logs run_YYYYMMDDTHHMMSS
go run ./cmd/trybox stop
```

Build a local binary:

```sh
go build -o trybox ./cmd/trybox
./trybox doctor
```

## Targets

Built-in targets:

- `macos15-arm64`: default macOS 15 Apple Silicon target
- `macos15-x64-rosetta`: macOS 15 target for x64 behavior through Rosetta
- macOS 10.15 and 14 targets are reference-only until Trybox has a backend
  that can reproduce them locally.

These targets are local OS and architecture shapes. They do not expose production
pooling concepts, create pools, or talk to Taskcluster.

## Target Images

`trybox up` expects a local target image that is already SSH-ready. That image
should be owned by Trybox, even if the first version is bootstrapped from a
public Tart image.

The image layers are:

- source image: immutable seed image, eventually hosted by Trybox
- target image: local golden image for a target such as `macos15-arm64`
- workspace VM: disposable clone used for one repo-bound workspace

The planned first-time setup command is:

```sh
trybox bootstrap --target macos15-arm64
```

## Commands

```sh
trybox doctor [--json]
trybox target list [--json]
trybox up [--target name] [--repo path]
trybox sync [--target name] [--repo path] [--json]
trybox status [--target name] [--repo path] [--json]
trybox run [--target name] [--repo path] -- <command>
trybox history [--limit n] [--json]
trybox logs <run-id>
trybox events <run-id> [--json]
trybox sync-plan [--repo path] [--limit n] [--json]
trybox stop [--target name] [--repo path]
trybox destroy [--target name] [--repo path]
```

## Design Goals

- Use clean, disposable OS workspaces for debugging real Firefox failures.
- Keep the public model target/workspace/run based, not backend based.
- Make every long-running command resumable through durable logs.
- Keep agent-facing output machine-readable with `--json`.
- Sync tracked and nonignored local source into the guest so dirty Firefox
  worktrees can be tested without making agents manage VM internals.
- Keep future task import as metadata and planning first, execution second.

## Non-Goals

- No production provisioning in local repro VMs.
- No CI registration or pool management.
- No secrets forwarding by default.
- No full reimplementation of Tart or Apple Virtualization.Framework in the
  first phase.

See [docs/architecture.md](docs/architecture.md) and
[docs/security.md](docs/security.md). Image ownership and bootstrap strategy
are covered in [docs/images.md](docs/images.md).
