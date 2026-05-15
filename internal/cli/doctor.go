package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/targets"
)

func doctor(ctx context.Context, args []string) error {
	fs, opts := commandFlags("doctor", flagSpec{Target: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	_, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	target, err := targets.Get(targetNameFor(opts, config))
	if err != nil {
		return err
	}
	checks := localToolChecks()
	checks = append(checks, backendFor(target).Doctor(ctx, target)...)
	if opts.JSON {
		if err := writeJSON(os.Stdout, checks); err != nil {
			return err
		}
		if !allChecksOK(checks) {
			return exitError{Code: 2}
		}
		return nil
	}
	ok := true
	for _, check := range checks {
		mark := "ok"
		if !check.OK {
			mark = "fail"
			ok = false
		}
		fmt.Printf("%-8s %-16s %s\n", mark, check.Name, check.Detail)
	}
	if !ok {
		return exitError{Code: 2}
	}
	return nil
}

func localToolChecks() []backend.Check {
	tools := []string{"git", "rsync", "ssh", "ssh-keygen"}
	checks := make([]backend.Check, 0, len(tools))
	for _, tool := range tools {
		if path, err := exec.LookPath(tool); err == nil {
			checks = append(checks, backend.Check{Name: tool, OK: true, Detail: path})
		} else {
			checks = append(checks, backend.Check{Name: tool, OK: false, Detail: tool + " not found in PATH"})
		}
	}
	return checks
}

func allChecksOK(checks []backend.Check) bool {
	for _, check := range checks {
		if !check.OK {
			return false
		}
	}
	return true
}

func target(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "list" {
		if len(args) == 0 || isHelp(args[0]) {
			fmt.Fprint(os.Stdout, commandUsage("target"))
			return nil
		}
		return fmt.Errorf("usage: trybox target list [--json]")
	}
	fs := flag.NewFlagSet("target list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprint(os.Stdout, commandUsage("target list"))
	}
	jsonOut := fs.Bool("json", false, "emit JSON")
	if handled, err := parseFlags(fs, args[1:]); handled || err != nil {
		return err
	}
	list := targets.List()
	views := make([]targetView, 0, len(list))
	for _, target := range list {
		views = append(views, viewTarget(target, backendFor(target).Exists(ctx, target.ImageName)))
	}
	if *jsonOut {
		return writeJSON(os.Stdout, views)
	}
	for _, target := range views {
		runnable := "reference"
		if target.Runnable {
			runnable = "runnable"
		}
		image := "missing"
		if target.ImagePresent {
			image = "present"
		}
		fmt.Printf("%-26s %-9s %-8s %-10s %-8s %s\n", target.Name, runnable, target.OS, target.Version, target.Arch, image)
		if target.BootstrapCommand != "" {
			fmt.Printf("  bootstrap: %s\n", target.BootstrapCommand)
		}
	}
	_ = ctx
	return nil
}
