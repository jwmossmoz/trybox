# Trybox

Trybox creates clean local debugging workspaces for Mozilla product
development.

The name is a nod to Mozilla's "try" workflow: a local box for trying source
changes before sending work elsewhere.

The first backend is Tart on Apple Silicon macOS. It starts a clean macOS VM
target, syncs a selected local source checkout, runs commands inside the guest,
and stores durable logs for humans and agents.

## Quick Start

Prerequisites:

- Apple Silicon macOS host
- Tart installed
- A Trybox macOS target image with SSH enabled
- A Mozilla source checkout, selected with `trybox workspace use` or passed
  with `--repo`

The examples below use `~/mozilla-unified`; pass another checkout path, such as
`~/comm-central`, when working on a different product.

```sh
go run ./cmd/trybox doctor
go run ./cmd/trybox target list
go run ./cmd/trybox workspace use ~/mozilla-unified
go run ./cmd/trybox up
go run ./cmd/trybox sync
go run ./cmd/trybox run -- ./mach --version
go run ./cmd/trybox view
go run ./cmd/trybox logs run_YYYYMMDDTHHMMSS
go run ./cmd/trybox stop
```

`trybox view` verifies desktop auto-login and restarts the workspace VM with the
native Tart display window. Use `trybox view --vnc` only when you specifically
want macOS Screen Sharing. Early Tart-seeded images use `admin` / `admin`;
`trybox bootstrap` should eventually bake a Trybox-owned guest user into target
images so visual smoke tests never stop at the login window.

`trybox view --vnc --no-open` starts Tart's VNC mode and prints the connection
details without launching Apple's Screen Sharing client.

Build a local binary:

```sh
go build -o trybox ./cmd/trybox
./trybox doctor
```

## Targets

Built-in targets:

- `macos12-arm64` / `macos12-x64-rosetta`
- `macos13-arm64` / `macos13-x64-rosetta`
- `macos14-arm64` / `macos14-x64-rosetta`
- `macos15-arm64` / `macos15-x64-rosetta`
- `macos26-arm64` / `macos26-x64-rosetta`

These targets are local OS and architecture shapes. They do not expose
backend image details in the normal workspace workflow.

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
trybox workspace use [--target name] [--cpu n] [--memory-mb n] [--disk-gb n] [repo]
trybox workspace show [--json]
trybox workspace clear
trybox up [--target name] [--repo path]
trybox sync [--target name] [--repo path] [--json]
trybox status [--target name] [--repo path] [--json]
trybox run [--target name] [--repo path] -- <command>
trybox view [--target name] [--repo path] [--vnc] [--no-open] [--reuse-client] [--json]
trybox history [--limit n] [--json]
trybox logs <run-id>
trybox events <run-id> [--json]
trybox sync-plan [--repo path] [--limit n] [--json]
trybox stop [--target name] [--repo path]
trybox destroy [--target name] [--repo path]
```

## Design Goals

- Use clean, disposable OS workspaces for debugging real Mozilla product
  failures.
- Keep the public model target/workspace/run based, not backend based.
- Make every long-running command resumable through durable logs.
- Keep agent-facing output machine-readable with `--json`.
- Sync tracked files, repository metadata, and nonignored local source into the
  guest so dirty Mozilla worktrees can be tested without making agents manage
  VM internals.

See [docs/architecture.md](docs/architecture.md) and
[docs/security.md](docs/security.md). Image ownership and bootstrap strategy
are covered in [docs/images.md](docs/images.md).

Agent instructions live in [AGENTS.md](AGENTS.md). Repo-local agent skills live
under [.agents/skills](.agents/skills).
