package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
)

func isHelp(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

func printCommandHelp(parts []string) error {
	if len(parts) == 0 {
		usage(os.Stdout)
		return nil
	}
	name := strings.Join(parts, " ")
	text := commandUsage(name)
	if text == "" {
		return fmt.Errorf("unknown help topic %q", name)
	}
	fmt.Fprint(os.Stdout, text)
	return nil
}

func commandUsage(name string) string {
	usages := map[string]string{
		"destroy": `trybox destroy: delete only the selected workspace VM

Usage:
  trybox destroy [<workspace-id>] [--json]

Notes:
  Without an id, destroys the configured default workspace VM.
  Use trybox workspace list to see available workspace ids.
  Does not delete the host checkout, run logs, or workspace metadata.
`,
		"doctor": `trybox doctor: check host tools and the selected target image

Usage:
  trybox doctor [--target name] [--json]
`,
		"events": `trybox events: print a run event stream

Usage:
  trybox events <run-id> [--json]
`,
		"history": `trybox history: list recent runs

Usage:
  trybox history [--limit n] [--json]
`,
		"logs": `trybox logs: print stdout and stderr logs for a run

Usage:
  trybox logs <run-id>
`,
		"run": `trybox run: sync the workspace and run a command in the guest

Usage:
  trybox run [--target name] [--repo path] [--json] -- <command>
`,
		"snapshot": `trybox snapshot: save, restore, list, and delete workspace VM snapshots

Usage:
  trybox snapshot save <name> [--target name] [--repo path] [--json]
  trybox snapshot list [--target name] [--repo path] [--json]
  trybox snapshot restore <name> [--display] [--target name] [--repo path] [--json]
  trybox snapshot delete <name> [--target name] [--repo path] [--json]

Notes:
  Snapshot names must be kebab-case. Snapshots are scoped to the selected
  workspace and keep metadata under Trybox state.
`,
		"snapshot save": `trybox snapshot save: capture the selected workspace VM state

Usage:
  trybox snapshot save <name> [--target name] [--repo path] [--json]
`,
		"snapshot list": `trybox snapshot list: list snapshots for the selected workspace

Usage:
  trybox snapshot list [--target name] [--repo path] [--json]
`,
		"snapshot restore": `trybox snapshot restore: replace the workspace VM from a snapshot

Usage:
  trybox snapshot restore <name> [--display] [--target name] [--repo path] [--json]
`,
		"snapshot delete": `trybox snapshot delete: delete a workspace snapshot

Usage:
  trybox snapshot delete <name> [--target name] [--repo path] [--json]
`,
		"status": `trybox status: show workspace VM state

Usage:
  trybox status [--target name] [--repo path] [--json]
`,
		"stop": `trybox stop: stop a workspace VM without deleting it

Usage:
  trybox stop [--target name] [--repo path] [--json]
`,
		"sync": `trybox sync: sync the source checkout into the guest workspace

Usage:
  trybox sync [--target name] [--repo path] [--json]
`,
		"sync-plan": `trybox sync-plan: preview the manifest and transfer size

Usage:
  trybox sync-plan [--repo path] [--limit n] [--json]
`,
		"target": `trybox target: inspect built-in target names

Usage:
  trybox target list [--json]
`,
		"target list": `trybox target list: list built-in target names

Usage:
  trybox target list [--json]
`,
		"up": `trybox up: create and start the workspace VM

Usage:
  trybox up [--target name] [--repo path] [--cpu n] [--memory-mb n] [--disk-gb n] [--json]
`,
		"view": `trybox view: open the workspace desktop

Usage:
  trybox view [--target name] [--repo path] [--vnc] [--no-open] [--reuse-client] [--restart-display] [--json]

Notes:
  Default display is Tart native. --vnc starts Tart VNC. --no-open does not open a host GUI client.
  --json implies --no-open. A running VM is not restarted unless --restart-display is passed.
`,
		"workspace": `trybox workspace: manage the default workspace

Usage:
  trybox workspace list [--json]
  trybox workspace show [--json]
  trybox workspace unset [--json]
  trybox workspace use [--target name] [--cpu n] [--memory-mb n] [--disk-gb n] [--json] [repo]
`,
		"workspace list": `trybox workspace list: list all known workspaces

Usage:
  trybox workspace list [--json]
`,
		"workspace unset": `trybox workspace unset: unset the default workspace pointer

Usage:
  trybox workspace unset [--json]
`,
		"workspace show": `trybox workspace show: show the configured default workspace

Usage:
  trybox workspace show [--json]
`,
		"workspace use": `trybox workspace use: set the default source checkout and target

Usage:
  trybox workspace use [--target name] [--cpu n] [--memory-mb n] [--disk-gb n] [--json] [repo]
`,
	}
	return usages[name]
}

func usage(w io.Writer) {
	fmt.Fprint(w, `trybox: clean local VM workspaces for source debugging

Usage:
  trybox destroy [<workspace-id>] [--json]
  trybox doctor [--target name] [--json]
  trybox events <run-id> [--json]
  trybox history [--limit n] [--json]
  trybox logs <run-id>
  trybox run [--target name] [--repo path] [--json] -- <command>
  trybox snapshot save <name> [--target name] [--repo path] [--json]
  trybox snapshot list [--target name] [--repo path] [--json]
  trybox snapshot restore <name> [--display] [--target name] [--repo path] [--json]
  trybox snapshot delete <name> [--target name] [--repo path] [--json]
  trybox status [--target name] [--repo path] [--json]
  trybox stop [--target name] [--repo path] [--json]
  trybox sync [--target name] [--repo path] [--json]
  trybox sync-plan [--repo path] [--limit n] [--json]
  trybox target list [--json]
  trybox up [--target name] [--repo path] [--cpu n] [--memory-mb n] [--disk-gb n] [--json]
  trybox view [--target name] [--repo path] [--vnc] [--no-open] [--reuse-client] [--restart-display] [--json]
  trybox workspace list [--json]
  trybox workspace show [--json]
  trybox workspace unset [--json]
  trybox workspace use [--target name] [--cpu n] [--memory-mb n] [--disk-gb n] [--json] [repo]

MVP backend:
  macOS targets via Tart.

Run "trybox help <command>" or "trybox <command> --help" for command help.
`)
}
