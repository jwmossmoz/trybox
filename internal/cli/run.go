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
	fs, opts := commandFlags("run", flagSpec{Target: true, Repo: true, JSON: true, Resources: true})
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
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	return withWorkspaceLock(ctx, store, workspace.ID, func() error {
		run, err := store.NewRun(workspace, command)
		if err != nil {
			return err
		}
		_ = store.AppendEvent(run, "run_created", map[string]any{"command": command})
		if err := store.SaveRun(run); err != nil {
			return err
		}
		if !opts.JSON {
			fmt.Fprintln(os.Stderr, "run context:")
			fmt.Fprintf(os.Stderr, "  run=%s logs=trybox logs %s events=trybox events %s\n", run.ID, run.ID, run.ID)
			fmt.Fprintf(os.Stderr, "  target=%s vm=%s repo=%s\n", workspace.Target, workspace.VMName, workspace.RepoRoot)
			fmt.Fprintf(os.Stderr, "  workdir=%s\n", remoteWorkPath(target))
			fmt.Fprintf(os.Stderr, "vm:        ensuring %s\n", workspace.VMName)
		}
		_ = store.AppendEvent(run, "vm_ensure_started", nil)
		if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
			run.EndedAt = time.Now().UTC()
			run.ExitCode = -1
			_ = store.AppendEvent(run, "run_failed", map[string]any{"phase": "vm_ensure", "error": err.Error()})
			_ = store.SaveRun(run)
			return err
		}
		_ = store.AppendEvent(run, "vm_ensure_finished", map[string]any{"vm_name": workspace.VMName, "ip": workspace.LastKnownIP})
		if !opts.JSON {
			fmt.Fprintf(os.Stderr, "vm:        running %s ip=%s\n", workspace.VMName, workspace.LastKnownIP)
			fmt.Fprintf(os.Stderr, "sync:      preparing checkout\n")
		}
		syncResult, err := syncWorkspaceState(ctx, target, &workspace, b, store, &run, os.Stderr)
		if err != nil {
			run.EndedAt = time.Now().UTC()
			run.ExitCode = -1
			_ = store.AppendEvent(run, "run_failed", map[string]any{"phase": "sync", "error": err.Error()})
			_ = store.SaveRun(run)
			return err
		}

		output, err := os.OpenFile(run.OutputLog, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return err
		}
		defer output.Close()
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
		if !opts.JSON {
			fmt.Fprintf(os.Stderr, "command:   %s\n", shellJoin(command))
		}
		commandStarted := time.Now()
		commandStdout := io.Writer(stdout)
		if !opts.JSON {
			commandStdout = io.MultiWriter(os.Stdout, stdout)
		}
		commandStdout = io.MultiWriter(commandStdout, output)
		exitCode, execErr := b.Exec(ctx, target, workspace, command, backend.ExecOptions{
			Workdir: remoteWorkPath(target),
			Stdout:  commandStdout,
			Stderr:  io.MultiWriter(os.Stderr, stderr, output),
		})
		commandDuration := time.Since(commandStarted).Round(time.Millisecond)
		run.EndedAt = time.Now().UTC()
		run.ExitCode = exitCode
		_ = store.AppendEvent(run, "command_finished", map[string]any{"exit_code": exitCode})
		if err := store.SaveRun(run); err != nil {
			return err
		}
		workspace.LastRunLog = store.RunDir(run.ID)
		_ = store.SaveWorkspace(workspace)
		runOut := viewRun(run)
		runOut.Sync = &syncResult
		runOut.CommandDuration = commandDuration.String()
		if opts.JSON {
			_ = writeJSON(os.Stdout, runOut)
		}
		if !opts.JSON {
			fmt.Fprintf(os.Stderr, "summary:   sync=%s command=%s total=%s exit=%d\n", syncResult.Duration, commandDuration, runOut.Duration, exitCode)
			fmt.Fprintf(os.Stderr, "logs:      trybox logs %s\n", run.ID)
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
	jsonOut := false
	runID := ""
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(arg, "-") || runID != "" {
				return fmt.Errorf("usage: trybox logs [run-id] [--json]")
			}
			runID = arg
		}
	}
	store, err := state.DefaultStore()
	if err != nil {
		return err
	}
	if runID == "" {
		runID, err = latestRunID(store)
		if err != nil {
			return err
		}
	}
	output, err := readRunOutput(store, runID)
	if err != nil {
		return err
	}
	if jsonOut {
		out := logView{
			RunID:     runID,
			Output:    string(output),
			OutputLog: filepath.Join(store.RunDir(runID), "output.log"),
			StdoutLog: filepath.Join(store.RunDir(runID), "stdout.log"),
			StderrLog: filepath.Join(store.RunDir(runID), "stderr.log"),
		}
		if run, err := readRun(store, runID); err == nil {
			view := viewRun(run)
			out.Run = &view
		}
		_ = ctx
		return writeJSON(os.Stdout, out)
	}
	if len(output) == 0 {
		_ = ctx
		return nil
	}
	_, err = os.Stdout.Write(output)
	_ = ctx
	return err
}

func readRunOutput(store state.Store, runID string) ([]byte, error) {
	if data, err := os.ReadFile(filepath.Join(store.RunDir(runID), "output.log")); err == nil {
		return data, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	var output []byte
	for _, name := range []string{"stdout.log", "stderr.log"} {
		data, err := os.ReadFile(filepath.Join(store.RunDir(runID), name))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, err
		}
		output = append(output, data...)
	}
	return output, nil
}

func readRun(store state.Store, runID string) (state.Run, error) {
	data, err := os.ReadFile(filepath.Join(store.RunDir(runID), "meta.json"))
	if err != nil {
		return state.Run{}, err
	}
	var run state.Run
	if err := json.Unmarshal(data, &run); err != nil {
		return state.Run{}, err
	}
	return run, nil
}

func latestRunID(store state.Store) (string, error) {
	entries, err := os.ReadDir(store.RunsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("no runs found")
		}
		return "", err
	}
	var latest state.Run
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		run, err := readRun(store, entry.Name())
		if err != nil {
			continue
		}
		if latest.ID == "" || run.StartedAt.After(latest.StartedAt) {
			latest = run
		}
	}
	if latest.ID == "" {
		return "", fmt.Errorf("no runs found")
	}
	return latest.ID, nil
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
	if !jsonOut {
		for _, event := range out {
			fmt.Printf("%-30s %-20s %-8s %s\n", stringField(event, "ts"), stringField(event, "type"), stringField(event, "phase"), eventPayloadSummary(event))
		}
		_ = ctx
		return nil
	}
	_ = ctx
	return writeJSON(os.Stdout, out)
}

func stringField(value map[string]any, name string) string {
	if s, ok := value[name].(string); ok {
		return s
	}
	return ""
}

func eventPayloadSummary(event map[string]any) string {
	payload, ok := event["payload"]
	if !ok || payload == nil {
		return ""
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
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
