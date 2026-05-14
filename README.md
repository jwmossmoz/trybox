# Trybox

Run a dirty checkout in a clean local VM.

```sh
cd ~/src/project
trybox run -- ./build-or-test-command test
```

Trybox syncs your current source checkout into a repo-bound local VM, runs the
command you pass, streams output, and keeps durable logs/events. The first
backend is Tart on Apple Silicon macOS.

## First-Time Setup

Prerequisites:

- Apple Silicon macOS host
- Go
- Tart
- Source checkout on the host
- SSH-ready Trybox target image

Install Trybox:

```sh
go install github.com/jwmossmoz/trybox/cmd/trybox@main
```

When working inside this repository, this is also fine:

```sh
go install ./cmd/trybox
```

Create the default Tart image until `trybox bootstrap` exists:

```sh
tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image
```

Check the host and target image:

```sh
trybox doctor
```

## Daily Use

Run from the checkout you want to test:

```sh
cd ~/src/project
trybox run -- ./build-or-test-command test
```

That is the main workflow. `trybox run -- <command>` selects the repo, selects
the target, starts the VM if needed, syncs the checkout, runs the command in the
guest, streams output, and records logs/events.

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
'Use Trybox for <checkout>: install missing deps (`brew install go cirruslabs/cli/tart`; `go install github.com/jwmossmoz/trybox/cmd/trybox@main`), set `TRYBOX_REPO=<checkout> TRYBOX_TARGET=macos15-arm64`, run `trybox doctor --json` and if the image is missing run `tart clone ghcr.io/cirruslabs/macos-sequoia-base:latest trybox-macos15-arm64-image`, then run `trybox run -- <command>`; report `trybox logs` and `trybox events <run-id> --json`; open `trybox view` or `trybox view --vnc` only if asked.'
```

## Commands

```sh
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
