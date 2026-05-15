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
		"bootstrap": `trybox bootstrap: create the local target image

Usage:
  trybox bootstrap [--target name] [--replace] [--json]

Notes:
  Early beta behavior clones the target's Cirrus Labs source image into
  Trybox's local target image name. Later this command can switch to
  Trybox-owned images without changing the setup flow.
  trybox run also bootstraps the target image automatically when missing.
`,
		"destroy": `trybox destroy: delete the VM for the selected repo and target

Usage:
  trybox destroy [--target name] [--repo path] [--json]

Notes:
  Does not delete the host checkout, run logs, or Trybox metadata.
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
		"logs": `trybox logs: print command output for a run

Usage:
  trybox logs [run-id] [--json]

Notes:
  Without a run id, prints the latest run log.
`,
		"run": `trybox run: sync the current checkout and run a command in the VM

Usage:
  trybox run [--target name] [--repo path] [--cpu n] [--memory-mb n] [--disk-gb n] [--json] -- <command>

Defaults:
  target: --target, then TRYBOX_TARGET, then saved config, then macos15-arm64.
  repo:   --repo, then TRYBOX_REPO, then the current git checkout.
  If the target image is missing, run bootstraps it automatically.
`,
		"status": `trybox status: show VM state

Usage:
  trybox status [--target name] [--repo path] [--json]
`,
		"target": `trybox target: inspect built-in target names

Usage:
  trybox target list [--json]
`,
		"target list": `trybox target list: list built-in target names

Usage:
  trybox target list [--json]
`,
		"view": `trybox view: open the VM desktop

Usage:
  trybox view [--target name] [--repo path] [--vnc] [--json]

Notes:
  Default display is Tart native. --vnc starts Tart's VNC server and prints the endpoint.
  VNC mode is for connecting with your own VNC client.
  Trybox restarts the VM when needed to switch display mode.
`,
	}
	return usages[name]
}

func usage(w io.Writer) {
	fmt.Fprint(w, `trybox: run a dirty checkout in a clean local VM

Usage:
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

MVP backend:
  macOS targets via Tart.

Environment:
  TRYBOX_TARGET, TRYBOX_REPO, TRYBOX_CPU, TRYBOX_MEMORY_MB, TRYBOX_DISK_GB

Run "trybox help <command>" or "trybox <command> --help" for command help.
`)
}
