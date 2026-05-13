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
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
	"github.com/jwmossmoz/trybox/internal/workspace"
)

type options struct {
	Target   string
	Repo     string
	JSON     bool
	Headless bool
}

type syncResult struct {
	RepoRoot    string `json:"repo_root"`
	RemotePath  string `json:"remote_path"`
	Fingerprint string `json:"fingerprint"`
	FileCount   int    `json:"file_count"`
	TotalBytes  int64  `json:"total_bytes"`
	Skipped     bool   `json:"skipped"`
	Duration    string `json:"duration"`
}

type targetView struct {
	Name     string `json:"name"`
	OS       string `json:"os"`
	Version  string `json:"version"`
	Arch     string `json:"arch"`
	Runnable bool   `json:"runnable"`
	Notes    string `json:"notes,omitempty"`
}

type claimView struct {
	ID              string    `json:"id"`
	Target          string    `json:"target"`
	RepoRoot        string    `json:"repo_root"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastRunLog      string    `json:"last_run_log,omitempty"`
	LastKnownIP     string    `json:"last_known_ip,omitempty"`
	SyncFingerprint string    `json:"sync_fingerprint,omitempty"`
	LastSyncAt      time.Time `json:"last_sync_at,omitempty"`
}

type runView struct {
	ID        string    `json:"id"`
	ClaimID   string    `json:"claim_id"`
	Target    string    `json:"target"`
	RepoRoot  string    `json:"repo_root"`
	Command   []string  `json:"command"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
	ExitCode  int       `json:"exit_code"`
	StdoutLog string    `json:"stdout_log"`
	StderrLog string    `json:"stderr_log"`
	EventsLog string    `json:"events_log"`
}

func Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		usage(os.Stdout)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		usage(os.Stdout)
		return nil
	case "doctor":
		return doctor(ctx, args[1:])
	case "target":
		return target(ctx, args[1:])
	case "warmup", "up":
		return warmup(ctx, args[1:])
	case "sync":
		return syncWorkspace(ctx, args[1:])
	case "status":
		return status(ctx, args[1:])
	case "stop":
		return stop(ctx, args[1:])
	case "destroy":
		return destroy(ctx, args[1:])
	case "run":
		return runCommand(ctx, args[1:])
	case "logs":
		return logs(ctx, args[1:])
	case "events":
		return events(ctx, args[1:])
	case "history":
		return history(ctx, args[1:])
	case "sync-plan":
		return syncPlan(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func Fatal(err error) {
	var exit exitError
	if errors.As(err, &exit) {
		os.Exit(exit.Code)
	}
	fmt.Fprintln(os.Stderr, "trybox:", err)
	os.Exit(1)
}

type exitError struct {
	Code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit %d", e.Code)
}

func baseFlags(name string, args []string) (*flag.FlagSet, *options) {
	opts := &options{Target: "macos15-arm64", Headless: true}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&opts.Target, "target", opts.Target, "target name")
	fs.StringVar(&opts.Repo, "repo", "", "repository root")
	fs.BoolVar(&opts.JSON, "json", false, "emit JSON")
	fs.BoolVar(&opts.Headless, "headless", true, "run VM without graphics")
	return fs, opts
}

func doctor(ctx context.Context, args []string) error {
	fs, opts := baseFlags("doctor", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	target, err := targets.Get(opts.Target)
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
		return fmt.Errorf("usage: trybox target list [--json]")
	}
	fs := flag.NewFlagSet("target list", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	list := targets.List()
	views := make([]targetView, 0, len(list))
	for _, target := range list {
		views = append(views, viewTarget(target))
	}
	if *jsonOut {
		return writeJSON(os.Stdout, views)
	}
	for _, target := range views {
		runnable := "reference"
		if target.Runnable {
			runnable = "runnable"
		}
		fmt.Printf("%-26s %-9s %-8s %-10s %s\n", target.Name, runnable, target.OS, target.Version, target.Arch)
	}
	_ = ctx
	return nil
}

func warmup(ctx context.Context, args []string) error {
	fs, opts := baseFlags("warmup", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	target, claim, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := ensureVM(ctx, target, &claim, b, store, opts.Headless); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, viewClaim(claim))
	}
	fmt.Printf("workspace: %s\ntarget:    %s\nip:        %s\nrepo:      %s\n", claim.ID, claim.Target, claim.LastKnownIP, claim.RepoRoot)
	return nil
}

func status(ctx context.Context, args []string) error {
	fs, opts := baseFlags("status", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, claim, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	exists := b.Exists(ctx, claim.VMName)
	running := b.IsRunning(ctx, claim.VMName)
	ip := ""
	if running {
		ip, _ = b.IP(ctx, claim, 1)
		claim.LastKnownIP = ip
		_ = store.SaveClaim(claim)
	}
	out := map[string]any{
		"workspace": viewClaim(claim),
		"exists":    exists,
		"running":   running,
		"ip":        ip,
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	fmt.Printf("workspace:   %s\ntarget:      %s\nexists:      %t\nrunning:     %t\nip:          %s\nrepo:        %s\nlast sync:   %s\n",
		claim.ID, claim.Target, exists, running, ip, claim.RepoRoot, claim.LastSyncAt.Format(time.RFC3339))
	return nil
}

func stop(ctx context.Context, args []string) error {
	fs, opts := baseFlags("stop", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, claim, b, _, err := setup(opts)
	if err != nil {
		return err
	}
	return b.Stop(ctx, claim)
}

func destroy(ctx context.Context, args []string) error {
	fs, opts := baseFlags("destroy", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, claim, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := b.Destroy(ctx, claim); err != nil {
		return err
	}
	return store.RemoveClaim(claim.ID)
}

func syncWorkspace(ctx context.Context, args []string) error {
	fs, opts := baseFlags("sync", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	target, claim, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := ensureVM(ctx, target, &claim, b, store, opts.Headless); err != nil {
		return err
	}
	result, err := syncClaim(ctx, target, &claim, b, store, nil)
	if err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, result)
	}
	action := "synced"
	if result.Skipped {
		action = "sync skipped"
	}
	fmt.Printf("%s: %d files, %s -> %s (%s)\n", action, result.FileCount, humanBytes(result.TotalBytes), result.RemotePath, result.Duration)
	return nil
}

func runCommand(ctx context.Context, args []string) error {
	fs, opts := baseFlags("run", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	command := fs.Args()
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}
	if len(command) == 0 {
		return fmt.Errorf("usage: trybox run [options] -- <command>")
	}
	target, claim, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := ensureVM(ctx, target, &claim, b, store, opts.Headless); err != nil {
		return err
	}
	run, err := store.NewRun(claim, command)
	if err != nil {
		return err
	}
	_ = store.AppendEvent(run, "run_created", map[string]any{"command": command})
	if err := store.SaveRun(run); err != nil {
		return err
	}
	if _, err := syncClaim(ctx, target, &claim, b, store, &run); err != nil {
		run.EndedAt = time.Now().UTC()
		run.ExitCode = -1
		_ = store.AppendEvent(run, "run_failed", map[string]any{"error": err.Error()})
		_ = store.SaveRun(run)
		return err
	}

	stdout, err := os.Create(run.StdoutLog)
	if err != nil {
		return err
	}
	defer stdout.Close()
	stderr, err := os.Create(run.StderrLog)
	if err != nil {
		return err
	}
	defer stderr.Close()

	_ = store.AppendEvent(run, "command_started", map[string]any{"command": command})
	exitCode, execErr := b.Exec(ctx, target, claim, command, backend.ExecOptions{
		Workdir: remoteWorkPath(target),
		Stdout:  io.MultiWriter(os.Stdout, stdout),
		Stderr:  io.MultiWriter(os.Stderr, stderr),
	})
	run.EndedAt = time.Now().UTC()
	run.ExitCode = exitCode
	_ = store.AppendEvent(run, "command_finished", map[string]any{"exit_code": exitCode})
	if err := store.SaveRun(run); err != nil {
		return err
	}
	claim.LastRunLog = store.RunDir(run.ID)
	_ = store.SaveClaim(claim)
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
}

func logs(ctx context.Context, args []string) error {
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
	fs := flag.NewFlagSet("events", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "emit JSON array")
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) != 1 {
		return fmt.Errorf("usage: trybox events <run-id> [--json]")
	}
	store, err := state.DefaultStore()
	if err != nil {
		return err
	}
	path := filepath.Join(store.RunDir(rest[0]), "events.ndjson")
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !*jsonOut {
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
	jsonOut := fs.Bool("json", false, "emit JSON")
	limit := fs.Int("limit", 20, "maximum runs to show")
	if err := fs.Parse(args); err != nil {
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

func syncPlan(ctx context.Context, args []string) error {
	fs, opts := baseFlags("sync-plan", args)
	limit := fs.Int("limit", 5, "largest files/directories to show")
	if err := fs.Parse(args); err != nil {
		return err
	}
	repo, err := resolveRepo(opts.Repo)
	if err != nil {
		return err
	}
	plan, err := workspace.BuildPlan(ctx, repo, *limit)
	if err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, plan)
	}
	fmt.Printf("repo: %s\nfiles: %d\nbytes: %s\nchanged tracked files: %d\nuntracked files: %d\nfingerprint: %s\n",
		repo, plan.FileCount, humanBytes(plan.TotalBytes), len(plan.ChangedTracked), len(plan.Untracked), plan.Fingerprint)
	if len(plan.LargestFiles) > 0 {
		fmt.Println("largest files:")
		for _, file := range plan.LargestFiles {
			fmt.Printf("  %8s %s\n", humanBytes(file.Size), file.Path)
		}
	}
	if len(plan.LargestDirs) > 0 {
		fmt.Println("largest directories:")
		for _, dir := range plan.LargestDirs {
			fmt.Printf("  %8s %s\n", humanBytes(dir.Size), dir.Path)
		}
	}
	return nil
}

func setup(opts *options) (targets.Target, state.Claim, backend.Backend, state.Store, error) {
	target, err := targets.Get(opts.Target)
	if err != nil {
		return targets.Target{}, state.Claim{}, nil, state.Store{}, err
	}
	repo, err := resolveRepo(opts.Repo)
	if err != nil {
		return targets.Target{}, state.Claim{}, nil, state.Store{}, err
	}
	store, err := state.DefaultStore()
	if err != nil {
		return targets.Target{}, state.Claim{}, nil, state.Store{}, err
	}
	if err := store.Init(); err != nil {
		return targets.Target{}, state.Claim{}, nil, state.Store{}, err
	}
	claimID := state.ClaimID(target.Name, repo)
	claim, err := store.LoadClaim(claimID)
	if err != nil {
		claim = state.Claim{
			ID:        claimID,
			Target:    target.Name,
			Backend:   target.Backend,
			VMName:    target.VMName,
			RepoRoot:  repo,
			CreatedAt: time.Now().UTC(),
		}
	}
	b := backendFor(target)
	return target, claim, b, store, nil
}

func ensureVM(ctx context.Context, target targets.Target, claim *state.Claim, b backend.Backend, store state.Store, headless bool) error {
	if err := b.Create(ctx, target, *claim); err != nil {
		return err
	}
	if err := b.Start(ctx, target, *claim, backend.StartOptions{Headless: headless}); err != nil {
		return err
	}
	ip, err := b.IP(ctx, *claim, 120)
	if err != nil {
		return err
	}
	claim.LastKnownIP = ip
	return store.SaveClaim(*claim)
}

func syncClaim(ctx context.Context, target targets.Target, claim *state.Claim, b backend.Backend, store state.Store, run *state.Run) (syncResult, error) {
	start := time.Now()
	plan, err := workspace.BuildPlan(ctx, claim.RepoRoot, 10)
	if err != nil {
		return syncResult{}, err
	}
	result := syncResult{
		RepoRoot:    claim.RepoRoot,
		RemotePath:  remoteWorkPath(target),
		Fingerprint: plan.Fingerprint,
		FileCount:   plan.FileCount,
		TotalBytes:  plan.TotalBytes,
	}
	if run != nil {
		_ = store.AppendEvent(*run, "sync_started", map[string]any{
			"file_count":   plan.FileCount,
			"total_bytes":  plan.TotalBytes,
			"fingerprint":  plan.Fingerprint,
			"remote_path":  result.RemotePath,
			"changed":      len(plan.ChangedTracked),
			"untracked":    len(plan.Untracked),
			"largest_dirs": plan.LargestDirs,
		})
	}

	if _, err := ensureSSHKey(ctx, target, *claim, b, store); err != nil {
		return result, err
	}
	if matches, err := remoteFingerprintMatches(ctx, target, *claim, b, plan.Fingerprint); err != nil {
		return result, err
	} else if matches {
		result.Skipped = true
		result.Duration = time.Since(start).Round(time.Millisecond).String()
		claim.SyncFingerprint = plan.Fingerprint
		claim.LastSyncAt = time.Now().UTC()
		if err := store.SaveClaim(*claim); err != nil {
			return result, err
		}
		if run != nil {
			_ = store.AppendEvent(*run, "sync_finished", result)
		}
		return result, nil
	}

	manifest, err := os.CreateTemp("", "trybox-manifest-*")
	if err != nil {
		return result, err
	}
	manifestPath := manifest.Name()
	defer os.Remove(manifestPath)
	if _, err := manifest.Write(plan.NULManifest()); err != nil {
		manifest.Close()
		return result, err
	}
	if err := manifest.Close(); err != nil {
		return result, err
	}

	keyPath := filepath.Join(store.KeyDir(claim.ID), "id_ed25519")
	ip, err := b.IP(ctx, *claim, 120)
	if err != nil {
		return result, err
	}
	sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR", shellQuote(keyPath))
	remote := fmt.Sprintf("%s@%s:%s/", target.Username, ip, remoteWorkPath(target))
	cmd := exec.CommandContext(ctx, "rsync",
		"-az",
		"--from0",
		"--files-from", manifestPath,
		"--relative",
		"-e", sshCmd,
		"./",
		remote,
	)
	cmd.Dir = claim.RepoRoot
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return result, fmt.Errorf("rsync failed: %w", err)
	}
	if err := writeRemoteFingerprint(ctx, target, *claim, b, plan.Fingerprint); err != nil {
		return result, err
	}
	result.Duration = time.Since(start).Round(time.Millisecond).String()
	claim.SyncFingerprint = plan.Fingerprint
	claim.LastSyncAt = time.Now().UTC()
	if err := store.SaveClaim(*claim); err != nil {
		return result, err
	}
	if run != nil {
		_ = store.AppendEvent(*run, "sync_finished", result)
	}
	return result, nil
}

func ensureSSHKey(ctx context.Context, target targets.Target, claim state.Claim, b backend.Backend, store state.Store) (string, error) {
	keyDir := store.KeyDir(claim.ID)
	keyPath := filepath.Join(keyDir, "id_ed25519")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return "", err
	}
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		cmd := exec.CommandContext(ctx, "ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath, "-C", "trybox-"+claim.ID)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("ssh-keygen failed: %w%s", err, suffix(strings.TrimSpace(string(out))))
		}
	}
	pub, err := os.ReadFile(keyPath + ".pub")
	if err != nil {
		return "", err
	}
	pubLine := strings.TrimSpace(string(pub))
	remote := remoteWorkPath(target)
	install := strings.Join([]string{
		"mkdir -p ~/.ssh " + shellQuote(remote) + " " + shellQuote(filepath.Join(remote, ".trybox")),
		"chmod 700 ~/.ssh",
		"touch ~/.ssh/authorized_keys",
		"(grep -qxF " + shellQuote(pubLine) + " ~/.ssh/authorized_keys || printf '%s\\n' " + shellQuote(pubLine) + " >> ~/.ssh/authorized_keys)",
		"chmod 600 ~/.ssh/authorized_keys",
	}, " && ")
	exitCode, err := b.Exec(ctx, target, claim, []string{"sh", "-lc", install}, backend.ExecOptions{
		Stdout: io.Discard,
		Stderr: os.Stderr,
	})
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		return "", fmt.Errorf("remote SSH key install failed with exit code %d", exitCode)
	}
	return keyPath, nil
}

func remoteFingerprintMatches(ctx context.Context, target targets.Target, claim state.Claim, b backend.Backend, fingerprint string) (bool, error) {
	check := "test \"$(cat " + shellQuote(remoteFingerprintPath(target)) + " 2>/dev/null)\" = " + shellQuote(fingerprint)
	exitCode, err := b.Exec(ctx, target, claim, []string{"sh", "-lc", check}, backend.ExecOptions{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		return false, err
	}
	return exitCode == 0, nil
}

func writeRemoteFingerprint(ctx context.Context, target targets.Target, claim state.Claim, b backend.Backend, fingerprint string) error {
	cmd := "mkdir -p " + shellQuote(filepath.Dir(remoteFingerprintPath(target))) + " && printf '%s\\n' " + shellQuote(fingerprint) + " > " + shellQuote(remoteFingerprintPath(target))
	exitCode, err := b.Exec(ctx, target, claim, []string{"sh", "-lc", cmd}, backend.ExecOptions{
		Stdout: io.Discard,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("remote fingerprint write failed with exit code %d", exitCode)
	}
	return nil
}

func remoteWorkPath(target targets.Target) string {
	if target.GuestWorkPath != "" {
		return target.GuestWorkPath
	}
	return filepath.Join("/Users", target.Username, "trybox", "work", "firefox")
}

func remoteFingerprintPath(target targets.Target) string {
	return filepath.Join(remoteWorkPath(target), ".trybox", "sync-fingerprint")
}

func viewTarget(target targets.Target) targetView {
	return targetView{
		Name:     target.Name,
		OS:       target.OS,
		Version:  target.Version,
		Arch:     target.Arch,
		Runnable: target.Runnable,
		Notes:    target.Notes,
	}
}

func viewClaim(claim state.Claim) claimView {
	return claimView{
		ID:              claim.ID,
		Target:          claim.Target,
		RepoRoot:        claim.RepoRoot,
		CreatedAt:       claim.CreatedAt,
		UpdatedAt:       claim.UpdatedAt,
		LastRunLog:      claim.LastRunLog,
		LastKnownIP:     claim.LastKnownIP,
		SyncFingerprint: claim.SyncFingerprint,
		LastSyncAt:      claim.LastSyncAt,
	}
}

func viewRun(run state.Run) runView {
	return runView{
		ID:        run.ID,
		ClaimID:   run.ClaimID,
		Target:    run.Target,
		RepoRoot:  run.RepoRoot,
		Command:   run.Command,
		StartedAt: run.StartedAt,
		EndedAt:   run.EndedAt,
		ExitCode:  run.ExitCode,
		StdoutLog: run.StdoutLog,
		StderrLog: run.StderrLog,
		EventsLog: run.EventsLog,
	}
}

func backendFor(target targets.Target) backend.Backend {
	switch target.Backend {
	case "tart":
		store, _ := state.DefaultStore()
		return backend.Tart{LogDir: store.LogsDir}
	case "reference":
		return backend.Reference{}
	default:
		return backend.Reference{}
	}
}

func resolveRepo(repo string) (string, error) {
	if repo != "" {
		return filepath.Abs(repo)
	}
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	defaultRepo := filepath.Join(home, "firefox")
	if _, statErr := os.Stat(defaultRepo); statErr == nil {
		return defaultRepo, nil
	}
	return "", fmt.Errorf("could not detect repo root; pass --repo")
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func suffix(detail string) string {
	if detail == "" {
		return ""
	}
	return ": " + detail
}

func usage(w io.Writer) {
	fmt.Fprint(w, `trybox: clean local Firefox debugging workspaces

Usage:
  trybox doctor [--json]
  trybox target list [--json]
  trybox up [--target name] [--repo path]
  trybox sync [--target name] [--repo path] [--json]
  trybox run [--target name] [--repo path] -- <command>
  trybox status [--target name] [--repo path] [--json]
  trybox history [--limit n] [--json]
  trybox logs <run-id>
  trybox events <run-id> [--json]
  trybox sync-plan [--repo path] [--limit n] [--json]
  trybox stop [--target name] [--repo path]
  trybox destroy [--target name] [--repo path]

MVP backend:
  macOS 15 arm64 via Tart. No production provisioning, no CI credentials, no CI registration.
`)
}
