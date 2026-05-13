# Trybox Image Model

Trybox should own the image path eventually. Public Tart images are useful as a
bootstrap seed, but they are not enough as the long-term foundation for
repeatable Mozilla product debugging.

## Terms

- **Source image**: an immutable image used to create a Trybox target image.
  Early development can use a public Tart image. Long term, this should come
  from a Trybox-owned registry and be pinned by digest.
- **Target image**: the local golden VM image for a Trybox target, such as
  `macos15-arm64`. `trybox up` clones this image into a workspace VM.
- **Workspace VM**: a disposable, repo-bound clone used by `trybox up`,
  `trybox sync`, and `trybox run`.

```mermaid
flowchart LR
    source["source image<br/>public seed or Trybox-owned registry"] --> bootstrap["trybox bootstrap"]
    bootstrap --> target["local target image<br/>SSH-ready, Trybox-normalized"]
    target --> workspace["workspace VM<br/>repo-bound disposable clone"]
    workspace --> run["trybox run"]
```

## Why Own Source Images

Relying on a public `latest` image is acceptable for an early prototype, but it
creates avoidable debugging noise:

- `latest` can drift under the same name.
- SSH, Rosetta, Xcode command line tools, system settings, and update state can
  change outside Trybox.
- A public registry can disappear, rate-limit, or publish a breaking change.
- Agents need a stable contract so a failing local repro can be repeated later.
- Security review is cleaner when the source image, bootstrap recipe, and digest
  are recorded together.

## Bootstrap Contract

`trybox bootstrap` should become the first-time setup command:

```sh
trybox bootstrap --target macos15-arm64
```

The command should:

1. Check host support and required tools.
2. Resolve the target's source image.
3. Create or refresh the local target image.
4. Start the target image and verify SSH.
5. Install Trybox's local SSH key.
6. Create the guest work directory.
7. Enable desktop auto-login for the Trybox guest user so `trybox view` opens
   directly into a usable desktop.
8. Record image metadata for `doctor`, `up`, and agents.

It should keep Tart details out of the normal `up/sync/run` workflow.

## Phases

Phase 0:

- Use a public Tart macOS base image as a seed.
- Create a local Trybox target image.
- Verify SSH and basic guest readiness.

Phase 1:

- Publish Trybox-owned source images to a private registry.
- Pin source images by digest.
- Store image metadata under Trybox state.
- Teach `doctor` to report the expected source digest and local target image
  status.

Phase 2:

- Add a repeatable image build recipe.
- Add `trybox image build`, `trybox image publish`, and `trybox image inspect`.
- Extend the same model to Windows targets.

## Initial Tart Seed

Tart's official quick start documents public macOS images, including
`ghcr.io/cirruslabs/macos-sequoia-base:latest`, and lists the default
`admin/admin` credentials for GUI, console, and SSH access:

```text
https://tart.run/quick-start/
```

That is enough for a first `trybox bootstrap`, but Trybox should treat it as a
seed, not as the stable product image.

## Tart macOS Seed Coverage

Trybox's built-in macOS target catalog should track the macOS families that Tart
publishes public base images for:

| Trybox targets | Tart seed image |
| --- | --- |
| `macos12-arm64`, `macos12-x64-rosetta` | `ghcr.io/cirruslabs/macos-monterey-base:latest` |
| `macos13-arm64`, `macos13-x64-rosetta` | `ghcr.io/cirruslabs/macos-ventura-base:latest` |
| `macos14-arm64`, `macos14-x64-rosetta` | `ghcr.io/cirruslabs/macos-sonoma-base:latest` |
| `macos15-arm64`, `macos15-x64-rosetta` | `ghcr.io/cirruslabs/macos-sequoia-base:latest` |
| `macos26-arm64`, `macos26-x64-rosetta` | `ghcr.io/cirruslabs/macos-tahoe-base:latest` |

Those seed names are an implementation aid for bootstrap. The normal agent
workflow should still use Trybox targets, not Tart image names.
