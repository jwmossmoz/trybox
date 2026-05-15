# Trybox

> [!WARNING]
> Trybox is early beta and expected to break. Use it with caution for now,
> especially around local VM state, source sync, and long-running commands.

Run a dirty checkout in a clean local VM. The first backend is Tart on Apple
Silicon macOS.

```sh
brew install go cirruslabs/cli/tart
go install github.com/jwmossmoz/trybox/cmd/trybox@main
cd ~/src/project
trybox run -- ./build-or-test-command test
```

That is the main workflow. `trybox run -- <command>` selects the repo, selects
the target, bootstraps the target image if it is missing, starts the VM if
needed, syncs the checkout, runs the command in the guest, streams output, and
records logs/events.

When working inside this repository, install local changes with:

```sh
go install ./cmd/trybox
```

Use `trybox bootstrap` only when you want to prefetch or refresh the target
image explicitly. For now, it clones the target's Cirrus Labs source image into
Trybox's local target image name.

```sh
trybox bootstrap
trybox bootstrap --replace
```

## After A Run

Useful follow-ups:

```sh
trybox logs                 # reprint the latest run log
trybox history              # list recent runs
trybox events <run-id>      # inspect run events
trybox status               # show the selected VM
trybox destroy              # delete the selected VM and start fresh next time
```

## Desktop Access

Open the VM with Tart's native window:

```sh
trybox view
```

Start Tart's VNC endpoint and print the URL for your own client:

```sh
trybox view --vnc
trybox view --vnc --json
```

`trybox view --vnc` does not launch Apple Screen Sharing. It starts Tart's VNC
mode, prints the localhost `vnc://` endpoint, and leaves client choice to you.

## Defaults

Trybox chooses defaults so the normal command stays short:

| Setting | Resolution order |
| --- | --- |
| Target | `--target`, then `TRYBOX_TARGET`, then saved config, then `macos15-arm64`. |
| Repo | `--repo`, then `TRYBOX_REPO`, then the current git checkout. |
| Resources | `--cpu`, `--memory-mb`, `--disk-gb`, then `TRYBOX_CPU`, `TRYBOX_MEMORY_MB`, `TRYBOX_DISK_GB`, then target defaults. |

Passing `--target` to a VM-backed command records it as the next default target.

Use another checkout or target without changing directories:

```sh
TRYBOX_REPO=~/src/project TRYBOX_TARGET=macos15-arm64 trybox run -- ./build-or-test-command
```

## Agents

When handing Trybox work to an agent, paste a single quoted prompt like this and
replace `<checkout>` and `<command>`:

```sh
'Use Trybox for <checkout>: install missing deps (`brew install go cirruslabs/cli/tart`; `go install github.com/jwmossmoz/trybox/cmd/trybox@main`), set `TRYBOX_REPO=<checkout> TRYBOX_TARGET=macos15-arm64`, then run `trybox run -- <command>`; Trybox bootstraps the target image if missing; report `trybox logs` and `trybox events <run-id> --json`; open `trybox view` or `trybox view --vnc` only if asked.'
```

## Commands

```sh
trybox bootstrap [--target name] [--replace] [--json]
trybox destroy [--target name] [--repo path] [--json]
trybox doctor [--target name] [--json]
trybox events <run-id> [--json]
trybox history [--limit n] [--json]
trybox logs [run-id] [--json]
trybox run [--target name] [--repo path] [--cpu n] [--memory-mb n] [--disk-gb n] [--json] -- <command>
trybox status [--target name] [--repo path] [--json]
trybox target list [--json]
trybox view [--target name] [--repo path] [--vnc] [--json]
```

## More Detail

- [Overview](docs/overview.md)
- [CLI guide](docs/cli.md)
- [Architecture](docs/architecture.md)
- [Image model](docs/images.md)
- [Security model](docs/security.md)
- [Agent instructions](AGENTS.md)
- [Trybox agent skill](.agents/skills/trybox/SKILL.md)
