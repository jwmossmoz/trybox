package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
)

func runCommand(ctx context.Context, args []string) error {
	fs, opts := commandFlags("run", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	command := fs.Args()
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}
	if len(command) == 0 {
		return fmt.Errorf("usage: trybox run [options] -- <command>")
	}
	return runWorkspaceCommand(ctx, opts, command, nil)
}

func runWorkspaceCommand(ctx context.Context, opts *options, command []string, extra map[string]any) error {
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	return withWorkspaceLock(ctx, store, workspace.ID, func() error {
		run, err := store.NewRun(workspace, command)
		if err != nil {
			return err
		}
		created := map[string]any{"command": command}
		for key, value := range extra {
			created[key] = value
		}
		_ = store.AppendEvent(run, "run_created", created)
		if err := store.SaveRun(run); err != nil {
			return err
		}
		_ = store.AppendEvent(run, "workspace_ensure_started", nil)
		if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
			run.EndedAt = time.Now().UTC()
			run.ExitCode = -1
			_ = store.AppendEvent(run, "run_failed", map[string]any{"phase": "workspace_ensure", "error": err.Error()})
			_ = store.SaveRun(run)
			return err
		}
		_ = store.AppendEvent(run, "workspace_ensure_finished", map[string]any{"vm_name": workspace.VMName, "ip": workspace.LastKnownIP})
		if _, err := syncWorkspaceState(ctx, target, &workspace, b, store, &run); err != nil {
			run.EndedAt = time.Now().UTC()
			run.ExitCode = -1
			_ = store.AppendEvent(run, "run_failed", map[string]any{"phase": "sync", "error": err.Error()})
			_ = store.SaveRun(run)
			return err
		}

		stdout, err := os.OpenFile(run.StdoutLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		defer stdout.Close()
		stderr, err := os.OpenFile(run.StderrLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		defer stderr.Close()

		_ = store.AppendEvent(run, "command_started", map[string]any{"command": command})
		commandStdout := io.Writer(stdout)
		if !opts.JSON {
			commandStdout = io.MultiWriter(os.Stdout, stdout)
		}
		exitCode, execErr := b.Exec(ctx, target, workspace, command, backend.ExecOptions{
			Workdir: remoteWorkPath(target),
			Stdout:  commandStdout,
			Stderr:  io.MultiWriter(os.Stderr, stderr),
		})
		run.EndedAt = time.Now().UTC()
		run.ExitCode = exitCode
		_ = store.AppendEvent(run, "command_finished", map[string]any{"exit_code": exitCode})
		if err := store.SaveRun(run); err != nil {
			return err
		}
		workspace.LastRunLog = store.RunDir(run.ID)
		_ = store.SaveWorkspace(workspace)
		if opts.JSON {
			_ = writeJSON(os.Stdout, viewRun(run))
		}
		if execErr != nil {
			return execErr
		}
		if exitCode != 0 {
			return exitError{Code: exitCode}
		}
		return nil
	})
}

func logs(ctx context.Context, args []string) error {
	if len(args) == 1 && isHelp(args[0]) {
		fmt.Fprint(os.Stdout, commandUsage("logs"))
		_ = ctx
		return nil
	}
	if len(args) != 1 {
		return fmt.Errorf("usage: trybox logs <run-id>")
	}
	store, err := state.DefaultStore()
	if err != nil {
		return err
	}
	for _, name := range []string{"stdout.log", "stderr.log"} {
		path := filepath.Join(store.RunDir(args[0]), name)
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if len(data) == 0 {
			continue
		}
		if _, err := os.Stdout.Write(data); err != nil {
			return err
		}
	}
	_ = ctx
	return nil
}

func events(ctx context.Context, args []string) error {
	if len(args) == 1 && isHelp(args[0]) {
		fmt.Fprint(os.Stdout, commandUsage("events"))
		_ = ctx
		return nil
	}
	jsonOut := false
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(arg, "-") || runID != "" {
				return fmt.Errorf("usage: trybox events <run-id> [--json]")
			}
			runID = arg
		}
	}
	if runID == "" {
		return fmt.Errorf("usage: trybox events <run-id> [--json]")
	}
	store, err := state.DefaultStore()
	if err != nil {
		return err
	}
	path := filepath.Join(store.RunDir(runID), "events.ndjson")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !jsonOut {
		_, err = os.Stdout.Write(data)
		_ = ctx
		return err
	}
	var out []map[string]any
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		var event map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return err
		}
		out = append(out, event)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	_ = ctx
	return writeJSON(os.Stdout, out)
}

func history(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprint(os.Stdout, commandUsage("history"))
	}
	jsonOut := fs.Bool("json", false, "emit JSON")
	limit := fs.Int("limit", 20, "maximum runs to show")
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	store, err := state.DefaultStore()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(store.RunsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	runs := []state.Run{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(store.RunDir(entry.Name()), "meta.json"))
		if err != nil {
			continue
		}
		var run state.Run
		if err := json.Unmarshal(data, &run); err == nil {
			runs = append(runs, run)
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].StartedAt.After(runs[j].StartedAt)
	})
	if *limit > 0 && len(runs) > *limit {
		runs = runs[:*limit]
	}
	if *jsonOut {
		views := make([]runView, 0, len(runs))
		for _, run := range runs {
			views = append(views, viewRun(run))
		}
		return writeJSON(os.Stdout, views)
	}
	for _, run := range runs {
		fmt.Printf("%-20s exit=%-4d %s %s\n", run.ID, run.ExitCode, run.StartedAt.Format(time.RFC3339), strings.Join(run.Command, " "))
	}
	_ = ctx
	return nil
}
