# Trybox Agent Instructions

This repository builds Trybox, a local clean-workspace runner for Mozilla
product development. Keep the public product model centered on:

- **target**: OS/version/architecture, such as `macos15-arm64`
- **workspace**: repo-bound VM clone for one target
- **run**: one command execution with durable logs and events

Keep user-facing CLI terminology local and product-oriented. Avoid
backend-specific details in the normal target/workspace/run workflow.

Do not describe Trybox as single-product. It should serve Mozilla product source
workspaces across browser, mail, and other repo-local build/test workflows
without naming any one product as the default.

## Working in This Repo

- Use `rg` for searching.
- Use `gofmt` on edited Go files.
- Run `go test ./...` before finishing code changes.
- Keep generated state, VM images, and large artifacts out of git.
- Prefer small, direct Go packages over broad abstractions until a backend
  boundary needs them.

## Agent Skill

Repo-local skills live under `.agents/skills/`.

Use `.agents/skills/trybox/SKILL.md` when an agent needs to use Trybox to
reproduce, debug, or verify Mozilla product behavior in a local clean VM
workspace.

The skill is for using Trybox as a tool. When editing Trybox itself, follow this
`AGENTS.md` first, then use the skill only when you need to exercise the CLI as
an agent workflow.

## Current Boundaries

- Tart is the first VM backend, but Trybox is not a Tart wrapper.
- `trybox up`, `trybox sync`, and `trybox run` should hide backend internals.
- `trybox bootstrap` is planned for first-time target image setup.
- Trybox target images should eventually be owned and pinned by Trybox.
- Public Tart images may be useful as bootstrap seeds, not as the long-term
  product contract.
- Source sync intentionally includes repository metadata such as `.git` or
  `.hg` so repo-local tooling sees a real checkout in the guest.
- `trybox view` is the supported desktop path; it should use Tart's native
  window by default and reserve VNC/Screen Sharing for explicit `--vnc` use.
- Bootstrap should make visual access simple by enabling auto-login for the
  Trybox guest user in target images.
