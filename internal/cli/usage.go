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
  trybox logs <run-id> [--follow|-f] [--from-end]

Notes:
  --follow keeps printing new log bytes until the run finishes, then exits with
  the run's recorded exit code. --from-end starts following at the end of
  existing logs and requires --follow.
`,
		"run": `trybox run: sync the workspace and run a command in the guest

Usage:
  trybox run [--target name] [--repo path] [--json] -- <command>
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
		"task": `trybox task: inspect or replay a Taskcluster task

Usage:
  trybox task <task-id> [run|shell] [--root-url URL] [--target name] [--repo path] [--json]

Notes:
  Without run or shell, prints the replay plan. --root-url can also come from TASKCLUSTER_ROOT_URL.
`,
		"target": `trybox target: inspect built-in target names

Usage:
  trybox target list [--json]
`,
		"try": `trybox try: inspect a source revision and optionally replay a task

Usage:
  trybox try <revision-or-url> [task <task-id> [run|shell]] [--root-url URL] [--target name] [--repo path] [--json]

Notes:
  Replay run/shell requires the host checkout HEAD to match the requested revision.
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
  trybox logs <run-id> [--follow|-f] [--from-end]
  trybox run [--target name] [--repo path] [--json] -- <command>
  trybox status [--target name] [--repo path] [--json]
  trybox stop [--target name] [--repo path] [--json]
  trybox sync [--target name] [--repo path] [--json]
  trybox sync-plan [--repo path] [--limit n] [--json]
  trybox task <task-id> [run|shell] [--root-url URL] [--target name] [--repo path] [--json]
  trybox target list [--json]
  trybox try <revision-or-url> [task <task-id> [run|shell]] [--root-url URL] [--target name] [--repo path] [--json]
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
