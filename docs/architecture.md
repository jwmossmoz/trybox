# Trybox Architecture

Trybox is a local execution control plane for clean source debugging. The first
backend is Tart, but the public model is not "a Tart wrapper." The public nouns
are:

- **Target**: an OS/architecture shape, such as `macos15-arm64`.
- **VM**: the repo-bound local machine for one target.
- **Run**: one command execution with durable logs and metadata.

## Layer Model

Trybox separates three concerns that are often conflated:

1. **Machine isolation**
   Full OS environments, such as Tart macOS VMs or future Windows VMs.

2. **Command isolation**
   Optional process-level sandboxing, such as Bubblewrap or nsjail for Linux.

3. **Workspace isolation**
   Source sync, secret filtering, controlled mounts, and artifact collection.

The macOS MVP uses a Tart VM for machine isolation and does not rely on macOS
process sandboxing as the main boundary.

## Components

```text
trybox CLI
  target registry
  state store
  run coordinator
  source sync
  backend interface
    tart backend
    future windows backend
    future linux/container backend
```

## Current Implementation Diagram

```mermaid
flowchart TD
    user["Human or agent"] --> cli["Trybox CLI"]

    cli --> targets["Target catalog<br/>macOS version + architecture"]
    cli --> lifecycle["VM lifecycle<br/>status / view / destroy"]
    cli --> run["Run coordinator<br/>start / sync / command"]

    targets --> backend["VM backend<br/>Tart today"]
    lifecycle --> backend
    backend --> vm["Clean local macOS VM"]

    run --> checkout["Host source checkout"]
    checkout --> workspace["Guest workspace<br/>~/trybox"]
    vm --> workspace
    run --> workspace

    cli --> state["Durable Trybox state<br/>workspaces, runs, logs, events, keys"]
    state --> user
```

## State

State lives under `~/.trybox`:

```text
~/.trybox/
  workspaces/
    workspace_*.json
  runs/
    run_*/
      meta.json
      output.log
      stdout.log
      stderr.log
      events.ndjson
  logs/
    <vm>.log
  keys/
    workspace_*/
      id_ed25519
```

Run logs are intentionally plain files so agents can recover after interruption.

## Backend Interface

The backend surface is intentionally small:

```go
type Backend interface {
    Doctor(...)
    Exists(...)
    IsRunning(...)
    Create(...)
    Start(...)
    Stop(...)
    Destroy(...)
    IP(...)
    Exec(...)
}
```

Tart is currently invoked through `os/exec`. Native Apple
Virtualization.Framework should only be considered if Tart blocks a critical
workflow.

## Target References

Trybox targets are local OS and architecture shapes. They can be chosen to
match the behavior a developer needs to reproduce, but the normal workflow
should stay target/run based instead of exposing backend image details.

The first implementation expects a Trybox macOS target image with SSH enabled.
Creating that target image is part of the Trybox setup story, not something
the agent-facing `run` flow should expose.

See [images.md](images.md) for the source image, target image, and repo VM
model.

## Large Repository Strategy

Each `trybox run` syncs the Git-managed working set and repository metadata
into the guest before executing the command:

```text
host <source checkout> -> guest ~/trybox
```

The intended large-repo sync path is intentionally simple:

1. Copy tracked files, nonignored untracked files, and VCS metadata into the
   guest with native `rsync`.
2. Keep `~/trybox` as a real checkout so repo-local tools can inspect Git or
   Mercurial state.
3. Warn on very large transfers instead of inventing a more complex source
   overlay too early.

The current rsync path records a fingerprint so unchanged worktrees can skip
repeated transfers. Sync transfer progress is emitted on stderr so command
stdout remains usable. Successful syncs also store the last remote manifest
under `~/trybox/.trybox/sync-manifest`; later syncs use it to remove files that
were deleted, renamed, or newly excluded on the host before writing the next
fingerprint.
