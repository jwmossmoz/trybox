# Trybox Agent Instructions

This repository builds Trybox, a local clean-VM runner for source debugging.
Keep the public model centered on:

- **target**: OS/version/architecture, such as `macos15-arm64`
- **VM**: repo-bound local machine for one target
- **run**: one command execution with durable logs and events

Keep user-facing CLI terminology local and source-oriented. Avoid
backend-specific details in the normal target/run workflow.

Do not describe Trybox as tied to one project or organization. It should serve
large source workspaces and repo-local build/test workflows without naming any
one checkout as the default.

## Working in This Repo

- Use `rg` for searching.
- Use `gofmt` on edited Go files.
- Run `go test ./...` before finishing code changes.
- Keep generated state, VM images, and large artifacts out of git.
- Prefer small, direct Go packages over broad abstractions until a backend
  boundary needs them.

## Documentation Ownership

- `README.md` is product-facing. Keep it focused on the promise, quickstart,
  common workflows, output contract, desktop access, defaults, and current
  limits.
- `docs/` is for longer reference material that would make the README harder to
  scan.
- `AGENTS.md` is for repository-maintainer instructions and product boundaries
  that agents must preserve while editing Trybox.
- `.agents/skills/trybox/SKILL.md` is a runbook for agents using Trybox as a
  tool against another checkout. Do not turn it into implementation
  documentation for this repository.

## Agent Skill

Repo-local skills live under `.agents/skills/`.

Use `.agents/skills/trybox/SKILL.md` when an agent needs to use Trybox to
reproduce, debug, or verify source behavior in a local clean VM.

The skill is for using Trybox as a tool. When editing Trybox itself, follow this
`AGENTS.md` first, then use the skill only when you need to exercise the CLI as
an agent workflow.

## Current Boundaries

- Tart is the first VM backend, but Trybox is not a Tart wrapper.
- `trybox run` should hide VM startup and source sync internals.
- Trybox runs exactly the command the user passes. Do not add hidden project or
  application launch behavior to `run`.
- `trybox bootstrap` is planned for first-time target image setup.
- Trybox target images should eventually be owned and pinned by Trybox.
- Public Tart images may be useful as bootstrap seeds, not as the long-term
  tool contract.
- Source sync intentionally includes repository metadata such as `.git` or
  `.hg` so repo-local tooling sees a real checkout in the guest.
- Current source sync follows tracked files, repository metadata, and nonignored
  local files. Ignored build artifacts are not assumed to exist in the guest
  unless the user explicitly includes or creates them.
- `trybox view` is the supported desktop path; it should use Tart's native
  window by default and reserve Tart VNC endpoint mode for explicit `--vnc` use.
- Bootstrap should make visual access simple by enabling auto-login for the
  Trybox guest user in target images.

## Integration Check Contract

- `ci/check-integration.sh` should run real `trybox` commands, not wrappers
  around Trybox behavior.
- Keep the script output mostly Trybox output. Script status lines should be
  short and only mark major phases.
- Use `jq` for JSON assertions in shell. Do not add Python just to parse Trybox
  JSON.
- Exercise both human-readable output and `--json` output for agent workflows.
- Exercise both desktop paths: `trybox view` for Tart native display and
  `trybox view --vnc` for Tart's VNC endpoint.
- The script's large-checkout default is a maintainer integration convenience,
  not part of Trybox's product model. Keep public terminology generic and
  source-oriented.
