# Trybox Spec

Trybox creates clean local VM workspaces for source debugging.

## MVP

- Go CLI.
- Tart backend for Apple Silicon macOS.
- Built-in macOS targets for the public macOS families Tart currently
  publishes.
- Trybox-owned macOS target image.
- Repo-bound VM workspaces.
- Durable run logs.
- JSON output for agent workflows.
- Manifest-based sync for tracked files, repository metadata, and nonignored
  local files.
- Sync planning before large transfers.
- Local target names instead of backend internals.

## Phase 1.5

- Incremental sync improvements for very large checkouts.
- Artifact collection.
- Screenshots.
- Command plan import and review.
- Target config files.
- Per-run timeout and network policy.
- Optional host helper sandboxing, with `sandbox-exec` only as best-effort
  defense in depth when available.

## Phase 2

- Windows backend.
- Windows targets for clean source debugging.
- Linux command sandbox backend for container-style workflows.

## Example Agent Flow

```sh
trybox doctor --json
cd ~/src/project
trybox workspace use --target macos15-arm64 --json
trybox up --json
trybox sync --json
trybox run -- ./build-or-test-command
trybox logs run_20260513T153000
trybox status --json
```
