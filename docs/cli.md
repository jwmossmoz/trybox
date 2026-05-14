# Trybox CLI Guide

This guide explains the public Trybox command surface and the concepts behind
it. The short version is:

```sh
cd ~/src/project
trybox workspace use --target macos15-arm64
trybox up
trybox sync-plan
trybox sync
trybox run -- ./build-or-test-command
trybox logs <run-id>
```

## Core Concepts

| Term | Meaning |
| --- | --- |
| Target | An OS/version/architecture shape, such as `macos15-arm64`. Targets are the names users pick. |
| Target image | The local reusable base image for a target. `trybox up` clones this image when it creates a workspace VM. |
| Source checkout | The repository directory on the host. This is the code Trybox syncs into the guest. |
| Workspace | Trybox state for one source checkout on one target. It records the target, source checkout path, workspace VM name, sync state, and run history links. |
| Workspace VM | The disposable VM clone attached to a workspace. It can be stopped, restarted, or destroyed without deleting the host checkout. |
| Sync plan | A preview of the files Trybox would copy into the guest, including tracked files, VCS metadata, and nonignored local files. |
| Run | One command execution inside the guest workspace. Runs have durable logs, metadata, and an event stream. |
| Run logs | Plain stdout/stderr files stored under Trybox state so they can be inspected after the command exits. |
| Run events | A timestamped event stream for run lifecycle steps such as sync start, command start, and command finish. |

## Normal Workflow

1. Check local prerequisites:

   ```sh
   trybox doctor
   trybox target list
   ```

2. Select the current source checkout and target:

   ```sh
   cd ~/src/project
   trybox workspace use --target macos15-arm64
   ```

   This creates or updates Trybox workspace state. It does not create a VM.

3. Create and start the workspace VM:

   ```sh
   trybox up
   ```

   `up` makes sure the workspace VM exists, starts it, waits for an IP address,
   and records that runtime state. If the VM already exists and is running,
   `up` is mostly a readiness check.

4. Preview the sync:

   ```sh
   trybox sync-plan
   ```

   `sync-plan` is read-only. It inspects the host checkout and shows the file
   count, byte count, changed tracked files, untracked files, large entries, and
   fingerprint. Use it before large first syncs.

5. Sync the checkout into the guest:

   ```sh
   trybox sync
   ```

   `sync` makes sure the workspace VM is running, copies the planned source set
   into the guest work path, records the sync fingerprint, and removes stale
   files that disappeared from the manifest.

6. Run a command in the guest:

   ```sh
   trybox run -- ./build-or-test-command test
   ```

   `run` makes sure the VM is running, syncs first, then executes the command in
   the guest checkout. Use `--` to separate Trybox flags from the command.

7. Inspect results:

   ```sh
   trybox history
   trybox events <run-id>
   trybox logs <run-id>
   trybox status
   ```

8. Stop or delete the VM:

   ```sh
   trybox stop
   trybox destroy
   ```

   `stop` keeps the workspace VM for later. `destroy` deletes the workspace VM
   and clears runtime state, but keeps the host checkout, run logs, and
   workspace metadata.

## Command Reference

| Command | What it does | VM impact | State impact |
| --- | --- | --- | --- |
| `trybox doctor` | Checks local tools and the selected target image. | Does not start a VM. | Read-only. |
| `trybox target list` | Lists built-in target names. | Does not start a VM. | Read-only. |
| `trybox workspace use [repo]` | Sets the default source checkout and target. | Does not start a VM. | Creates or updates workspace metadata and default workspace config. |
| `trybox workspace show` | Shows the default workspace. | Does not start a VM. | Read-only. |
| `trybox workspace list` | Lists known workspaces. | Does not start a VM. | Read-only. |
| `trybox workspace unset` | Clears the default workspace pointer. | Does not start a VM. | Updates config only. Existing workspaces remain. |
| `trybox up` | Creates and starts the selected workspace VM. | Creates the VM if missing, starts it if stopped, waits for IP. | Records last known IP and workspace resource settings. |
| `trybox status` | Shows whether the selected workspace VM exists and is running. | Does not start a VM. | May refresh last known IP when the VM is running. |
| `trybox sync-plan` | Previews the sync manifest and transfer size. | Does not start a VM. | Read-only. |
| `trybox sync` | Copies the host checkout into the guest workspace. | Creates/starts the VM if needed. | Records sync fingerprint and timestamp. |
| `trybox run -- <command>` | Syncs and runs a command inside the guest checkout. | Creates/starts the VM if needed. | Creates a run record, logs, events, and last-run pointer. |
| `trybox history` | Lists recent runs. | Does not start a VM. | Read-only. |
| `trybox events <run-id>` | Prints a run event stream. | Does not start a VM. | Read-only. |
| `trybox logs <run-id>` | Prints stdout and stderr logs for a run. | Does not start a VM. | Read-only. |
| `trybox view` | Opens the workspace desktop. | Creates/starts or restarts the VM depending on flags. | May record last known IP. |
| `trybox stop` | Stops the selected workspace VM. | Stops the VM if running. | Keeps workspace metadata and logs. |
| `trybox destroy [workspace-id]` | Deletes a workspace VM. | Stops and deletes the selected workspace VM. | Clears runtime fields but keeps workspace metadata and run logs. |

## Flag Reference

| Flag | Commands | Meaning |
| --- | --- | --- |
| `--target name` | `doctor`, `workspace use`, `up`, `status`, `stop`, `sync`, `run`, `view` | Selects the target, such as `macos15-arm64`. When omitted, Trybox uses the configured default target, then falls back to `macos15-arm64`. |
| `--json` | Most commands | Emits structured JSON for scripts and agents instead of human-readable text. |
| `--cpu n` | `workspace use`, `up` | Sets the CPU count for the workspace VM. Resource changes require destroying an existing VM first. |
| `--memory-mb n` | `workspace use`, `up` | Sets VM memory in MiB. Resource changes require destroying an existing VM first. |
| `--disk-gb n` | `workspace use`, `up` | Sets VM disk size in GiB. Resource changes require destroying an existing VM first. |
| `--limit n` | `history`, `sync-plan` | Limits output. For `history`, it limits runs. For `sync-plan`, it limits large-file and large-directory previews. |
| `--vnc` | `view` | Starts the workspace VM with Tart VNC display mode. |
| `--no-open` | `view` | Prints connection details without opening a host GUI client. `--json` implies `--no-open`. |
| `--reuse-client` | `view` | Accepted for compatibility with earlier command shapes. Trybox does not reset existing host GUI clients. |
| `--restart-display` | `view` | Stops a running VM and starts it again in the requested display mode. |

`--repo path` is still accepted by repo-bound commands as an explicit override,
but the normal workflow is to run Trybox from inside the source checkout. When
no workspace default exists, Trybox detects the current Git repository.

## Choosing Commands

Use `workspace use` when you want Trybox to remember a checkout and target.
Use `up` when you want the VM created and ready before doing anything else.
Use `sync-plan` when you want to understand what would transfer before touching
the VM. Use `sync` when you want the guest checkout updated but do not want to
run a command yet. Use `run` for the usual test/build path because it includes
the startup and sync steps.

Use `stop` when you want to keep the VM disk around. Use `destroy` when you
want the workspace VM removed and the next `up` to start from the target image
again. Neither command deletes the host checkout.

## Paths And State

Trybox stores state under `~/.trybox`:

```text
~/.trybox/
  workspaces/   workspace metadata
  runs/         run metadata, logs, and events
  logs/         backend VM logs
  keys/         per-workspace SSH keys
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

Commands that run guest processes may still write diagnostic output to stderr.
Automation should read structured results from stdout and treat stderr as
diagnostics.
