# Trybox Security Model

Trybox is designed for debugging real failures without turning a developer's
host into the execution environment.

## Defaults

- No SSH agent forwarding.
- No host home-directory mount.
- No environment variable forwarding except explicit future allowlists.
- Tart runs with clipboard sharing disabled.
- Source access is limited to the selected repo path.
- Imported command execution must be planned before it is run.

## SSH Trust Boundary

The macOS MVP connects to local, disposable repo VMs and currently disables
SSH host-key verification for guest command execution and rsync. That keeps the
first Tart flow simple, but it is a deliberate local-VM tradeoff rather than a
general remote-execution model. Before Trybox-managed target images carry
non-public credentials or run outside a local VM boundary, Trybox should pin the
guest host key on first contact under `~/.trybox/keys/<vm-id>/known_hosts`
and use that file for both SSH and rsync.

## Isolation Classes

Different tools provide different boundaries:

| Tool family | Boundary | Useful for Trybox | Not enough for |
| --- | --- | --- | --- |
| Tart / Apple Virtualization.Framework | Full macOS VM | Native macOS debugging | Process-level policy inside Linux |
| Windows Sandbox / Hyper-V | Full Windows environment | Phase 2 Windows targets | macOS reproduction |
| Bubblewrap | Linux process sandbox | Future Linux command sandboxing | macOS/Windows OS fidelity |
| nsjail | Linux process sandbox with seccomp/cgroups | Future stricter Linux command execution | macOS/Windows OS fidelity |
| Rootless Podman | Linux container | Container-style command reproduction | Native macOS/Windows workflows |
| gVisor | Sandboxed Linux container runtime | Stronger Linux container isolation | Native macOS/Windows workflows |
| Firecracker | Linux microVM | Future fast Linux VM backend | macOS GUI/native debugging |

## Why Tart First

The first problem is native macOS debugging for large source trees in a clean
machine. A process sandbox cannot reproduce OS version, GUI behavior, screen
capture, permissions, framework behavior, or Apple Silicon VM quirks. Tart
provides the machine boundary and lifecycle. Trybox provides the source-aware
workflow on top.

## Future Sandbox Layer

Trybox should eventually include a command sandbox interface:

```go
type CommandSandbox interface {
    Wrap(command []string, policy SandboxPolicy) ([]string, error)
}
```

That layer is primarily for Linux and imported command plans. It should not be
part of the macOS MVP unless a concrete use case requires it.

## macOS sandbox-exec

`sandbox-exec` is Apple’s legacy command-line entry point to the macOS
Seatbelt/App Sandbox policy engine. It constrains one process tree through an
SBPL policy while it still runs on the host OS and host kernel. It does not
provide a clean OS install, VM snapshots, guest filesystem state, Xcode state,
or machine-level isolation.

The local macOS man pages mark `sandbox-exec` and related `sandbox_init(3)`
policies as deprecated. Trybox must not require `sandbox-exec` for core
execution.

Permitted future use:

- Optional host-side defense in depth for narrow helper commands.
- Best-effort artifact/log parsing wrappers.
- Capability-detected, off by default, logged, and bypassable.

Not permitted as a core design:

- Replacing Tart VM isolation.
- Running source builds directly on the host as the main workflow.
- Depending on undocumented SBPL behavior for security or correctness.
