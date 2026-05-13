# Trybox Spec

Trybox creates secure local workspaces for CI-like Firefox development and
debugging.

## MVP

- Go CLI.
- Tart backend for Apple Silicon macOS.
- Built-in macOS 15 arm64 targets.
- Trybox-owned macOS target image.
- Repo-bound VM claims.
- Durable run logs.
- JSON output for agent workflows.
- Manifest-based sync for tracked and nonignored local files.
- Sync planning before large transfers.
- Local target names instead of production pooling concepts or backend
  internals.

## Phase 1.5

- Patch-over-seed source sync on top of an in-guest clean checkout.
- Artifact collection.
- Screenshots.
- Task import and plan.
- Target config files.
- Per-run timeout and network policy.
- Optional host helper sandboxing, with `sandbox-exec` only as best-effort
  defense in depth when available.

## Phase 2

- Windows backend.
- Windows targets matching CI-like Firefox Windows images.
- Linux command sandbox backend for Docker-style and task-debugger workflows.

## Example Agent Flow

```sh
trybox doctor --json
trybox up --target macos15-arm64 --repo ~/firefox --json
trybox sync --repo ~/firefox --json
trybox run --repo ~/firefox -- ./mach test path/to/test
trybox logs run_20260513T153000
trybox status --json
```
