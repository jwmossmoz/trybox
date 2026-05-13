# Trybox Security Model

Trybox is designed for debugging real failures without turning a developer's
host into the execution environment.

## Defaults

- No production provisioning.
- No CI credentials.
- No CI registration.
- No SSH agent forwarding.
- No host home-directory mount.
- No environment variable forwarding except explicit future allowlists.
- Tart runs with clipboard sharing disabled.
- Source access is limited to the selected repo path.
- Task payload execution must be planned before it is run.

## Isolation Classes

Different tools provide different boundaries:

| Tool family | Boundary | Useful for Trybox | Not enough for |
| --- | --- | --- | --- |
| Tart / Apple Virtualization.Framework | Full macOS VM | macOS CI-like debugging | Process-level policy inside Linux |
| Windows Sandbox / Hyper-V | Full Windows environment | Phase 2 Windows targets | macOS reproduction |
| Bubblewrap | Linux process sandbox | Future Linux command sandboxing | macOS/Windows OS fidelity |
| nsjail | Linux process sandbox with seccomp/cgroups | Future stricter Linux task execution | macOS/Windows OS fidelity |
| Rootless Podman | Linux container | Docker-style task reproduction | Native macOS/Windows tasks |
| gVisor | Sandboxed Linux container runtime | Stronger Linux container isolation | Native macOS/Windows tasks |
| Firecracker | Linux microVM | Future fast Linux VM backend | macOS GUI/native debugging |

## Why Tart First

The first problem is native macOS Firefox debugging in a clean machine. A
process sandbox cannot reproduce OS version, GUI behavior, screen capture,
permissions, framework behavior, or Apple Silicon VM quirks. Tart provides the
machine boundary and lifecycle. Trybox provides the Firefox-aware workflow on
top.

## Future Sandbox Layer

Trybox should eventually include a command sandbox interface:

```go
type CommandSandbox interface {
    Wrap(command []string, policy SandboxPolicy) ([]string, error)
}
```

That layer is primarily for Linux and imported task payloads. It should not be
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
- Running Firefox builds directly on the host as the main workflow.
- Depending on undocumented SBPL behavior for security or correctness.
