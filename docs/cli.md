# Trybox CLI Guide

This guide describes the canonical first-phase Trybox workflow. The short
version is:

```sh
cd ~/src/project
trybox run --target macos15-arm64 -- ./build-or-test-command
trybox logs
```

`trybox run` is the main command. It selects the repo and target, creates or
starts the VM, syncs the dirty checkout, runs the command in the guest, streams
output, and records durable logs/events.

## Core Concepts

| Term | Meaning |
| --- | --- |
| Target | An OS/version/architecture shape, such as `macos15-arm64`. |
| Target image | The local reusable base image for a target. The first run expects it to exist. |
| Source checkout | The repository directory on the host. Trybox syncs this into the guest. |
| VM | The repo-bound local VM for one source checkout and one target. |
| Run | One command execution inside the guest checkout, with durable logs and events. |

## Defaults

Trybox chooses defaults so the normal command stays short:

| Setting | Resolution order |
| --- | --- |
| Target | `--target`, then `TRYBOX_TARGET`, then saved config, then `macos15-arm64`. |
| Repo | `--repo`, then `TRYBOX_REPO`, then the current Git checkout. |
| Resources | `--cpu`, `--memory-mb`, `--disk-gb`, then `TRYBOX_CPU`, `TRYBOX_MEMORY_MB`, `TRYBOX_DISK_GB`, then target defaults. |

Passing `--target` to a VM-backed command records it as the next default target.

## Normal Workflow

1. Check prerequisites:

   ```sh
   trybox doctor
   trybox target list
   ```

2. Run a command in a clean VM:

   ```sh
   trybox run -- ./build-or-test-command test
   ```

3. Inspect results:

   ```sh
   trybox logs
   trybox history
   trybox events <run-id>
   trybox status
   ```

4. Open the desktop when needed:

   ```sh
   trybox view
   trybox view --vnc
   ```

5. Delete the VM when you want to start fresh:

   ```sh
   trybox destroy
   ```

## Command Reference

| Command | What it does | VM impact |
| --- | --- | --- |
| `trybox doctor` | Checks local tools and the selected target image. | Does not start a VM. |
| `trybox target list` | Lists built-in target names and target image status. | Does not start a VM. |
| `trybox run -- <command>` | Starts the VM if needed, syncs the checkout, streams the command, and prints phase/timing context. | Creates/starts the VM if needed. |
| `trybox logs [run-id] [--json]` | Prints the combined command log. `--json` includes content and log paths for agents. | Does not start a VM. |
| `trybox history` | Lists recent runs. | Does not start a VM. |
| `trybox events <run-id>` | Prints formatted run events; `--json` returns event records. | Does not start a VM. |
| `trybox status` | Shows whether the selected VM exists and is running. | Does not start a VM. |
| `trybox view` | Opens the VM through Tart's native display. | Restarts the VM in native display mode. |
| `trybox view --vnc` | Starts Tart's VNC server and prints the localhost endpoint. | Restarts the VM in Tart VNC mode. |
| `trybox destroy` | Deletes the selected VM and clears runtime state. | Stops and deletes the VM. |

## Flag Reference

| Flag | Commands | Meaning |
| --- | --- | --- |
| `--target name` | `doctor`, `run`, `status`, `view`, `destroy` | Selects the target, such as `macos15-arm64`. |
| `--repo path` | `run`, `status`, `view`, `destroy` | Selects the host checkout. |
| `--json` | Commands with structured output | Emits JSON instead of human-readable output. |
| `--cpu n` | `run` | Sets VM CPU count before VM creation. Existing VMs must be destroyed first. |
| `--memory-mb n` | `run` | Sets VM memory in MiB before VM creation. Existing VMs must be destroyed first. |
| `--disk-gb n` | `run` | Sets VM disk size in GiB before VM creation. Existing VMs must be destroyed first. |
| `--limit n` | `history` | Limits the number of runs shown. |
| `--vnc` | `view` | Uses Tart's VNC server and prints its generated localhost URL. |

## Paths And State

Trybox stores state under `~/.trybox`:

```text
~/.trybox/
  workspaces/   VM metadata
  runs/         run metadata, logs, and events
  logs/         backend VM logs
  keys/         per-VM SSH keys
```

The synced checkout lives at `/Users/admin/trybox` in the guest for the current
macOS targets.

Host shell expansion happens before Trybox sees a command. This means
`trybox run -- echo ~/trybox` expands `~` on the host. Use single quotes when
you want the guest shell to expand variables:

```sh
trybox run -- bash -lc 'cd "$HOME/trybox" && pwd'
```

## JSON Output

Use `--json` for scripts, automation, and agent workflows. JSON output is meant
to be stable enough for tooling, while human output can change to become more
readable.

Commands that run guest processes write Trybox phase/status lines and command
stderr to stderr. Guest stdout stays on stdout. Automation should read
structured results from stdout and treat stderr as diagnostics.
