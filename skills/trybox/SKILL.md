---
name: trybox
description: Use this skill whenever a user wants an agent to reproduce, debug, or verify Firefox behavior in a clean local CI-like workspace with Trybox. This includes requests to spin up a local macOS VM, sync a dirty Firefox checkout, run specific safe commands, inspect run logs, or compare local behavior with a macOS target. Prefer Trybox commands over direct Tart, SSH, or ad hoc VM commands.
---

# Trybox Skill

Trybox is a local clean-workspace runner for Firefox development. Use it as the
execution boundary for macOS CI-like debugging instead of running native test
commands directly on the host.

## Core rule

Do not call `tart` directly. Trybox owns VM lifecycle, sync, command execution,
logs, and state. If Trybox lacks a capability, report the gap instead of
bypassing it with raw VM commands.

## Standard workflow

1. Check the environment:

   ```sh
   trybox doctor --json
   trybox target list --json
   ```

2. Start or reuse the workspace:

   ```sh
   trybox up --target macos15-arm64 --repo ~/firefox --json
   ```

3. Inspect source sync impact before running:

   ```sh
   trybox sync --repo ~/firefox --json
   ```

4. Run a specific approved command:

   ```sh
   trybox run --repo ~/firefox -- ./mach test <path-or-suite>
   ```

5. Capture the run ID and inspect logs:

   ```sh
   trybox logs <run-id>
   trybox status --json
   ```

## Command policy

Prefer narrow commands that directly support the user's debugging request.

Allowed by default:

- `./mach ...`
- `mach ...`
- `python3 ...` when it runs a known repo script
- `git status`, `git diff`, `git log`
- read-only inspection commands such as `pwd`, `ls`, `find`, `cat`, `sed`, and
  `rg`

Avoid unless the user explicitly asks:

- shell wrappers such as `bash -lc ...`
- commands that modify global guest state
- package manager installs
- commands that access secrets or credentials
- direct SSH, direct Tart, or host-level virtualization commands

## Targets

Use `macos15-arm64` for the default macOS 15 Apple Silicon VM target.

Use `macos15-x64-rosetta` when debugging x64 behavior through Rosetta.

Other macOS targets may be reference-only. If a target is not runnable,
explain that Trybox knows the local target shape but does not yet have a
backend for that OS/architecture.

## Reporting

When finishing, include:

- target used
- command run
- run ID
- exit code
- relevant log path or summary
- any Trybox limitation encountered
