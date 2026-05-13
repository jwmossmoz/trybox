package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	neturl "net/url"
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
	Target    string
	TargetSet bool
	Repo      string
	JSON      bool
	Headless  bool
	VNC       bool
	CPU       int
	MemoryMB  int
	DiskGB    int
}

type syncResult struct {
	RepoRoot    string   `json:"repo_root"`
	RemotePath  string   `json:"remote_path"`
	Fingerprint string   `json:"fingerprint"`
	FileCount   int      `json:"file_count"`
	TotalBytes  int64    `json:"total_bytes"`
	Warnings    []string `json:"warnings,omitempty"`
	Skipped     bool     `json:"skipped"`
	Duration    string   `json:"duration"`
}

type targetView struct {
	Name     string `json:"name"`
	OS       string `json:"os"`
	Version  string `json:"version"`
	Arch     string `json:"arch"`
	Runnable bool   `json:"runnable"`
	Notes    string `json:"notes,omitempty"`
}

type workspaceView struct {
	ID              string    `json:"id"`
	Target          string    `json:"target"`
	RepoRoot        string    `json:"repo_root"`
	VMName          string    `json:"vm_name"`
	CPU             int       `json:"cpu,omitempty"`
	MemoryMB        int       `json:"memory_mb,omitempty"`
	DiskGB          int       `json:"disk_gb,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastRunLog      string    `json:"last_run_log,omitempty"`
	LastKnownIP     string    `json:"last_known_ip,omitempty"`
	SyncFingerprint string    `json:"sync_fingerprint,omitempty"`
	LastSyncAt      time.Time `json:"last_sync_at,omitempty"`
}

type runView struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Target      string    `json:"target"`
	RepoRoot    string    `json:"repo_root"`
	Command     []string  `json:"command"`
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at,omitempty"`
	ExitCode    int       `json:"exit_code"`
	StdoutLog   string    `json:"stdout_log"`
	StderrLog   string    `json:"stderr_log"`
	EventsLog   string    `json:"events_log"`
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
	case "workspace":
		return workspaceCommand(ctx, args[1:])
	case "warmup", "up":
		return warmup(ctx, args[1:])
	case "sync":
		return syncWorkspace(ctx, args[1:])
	case "status":
		return status(ctx, args[1:])
	case "view":
		return view(ctx, args[1:])
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
	opts := &options{Headless: true}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var(targetFlag{opts: opts}, "target", "target name")
	fs.StringVar(&opts.Repo, "repo", "", "repository root")
	fs.BoolVar(&opts.JSON, "json", false, "emit JSON")
	fs.BoolVar(&opts.Headless, "headless", true, "run VM without graphics")
	fs.BoolVar(&opts.VNC, "vnc", false, "start VM with VNC display")
	fs.IntVar(&opts.CPU, "cpu", 0, "override VM CPU count for this workspace")
	fs.IntVar(&opts.MemoryMB, "memory-mb", 0, "override VM memory in MiB for this workspace")
	fs.IntVar(&opts.DiskGB, "disk-gb", 0, "override VM disk size in GiB for this workspace")
	return fs, opts
}

type targetFlag struct {
	opts *options
}

func (f targetFlag) String() string {
	if f.opts == nil {
		return ""
	}
	return f.opts.Target
}

func (f targetFlag) Set(value string) error {
	f.opts.Target = value
	f.opts.TargetSet = true
	return nil
}

func doctor(ctx context.Context, args []string) error {
	fs, opts := baseFlags("doctor", args)
	if err := fs.Parse(args); err != nil {
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

func workspaceCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: trybox workspace use [repo] | show | clear")
	}
	switch args[0] {
	case "use":
		return workspaceUse(ctx, args[1:])
	case "show", "current":
		return workspaceShow(ctx, args[1:])
	case "clear":
		return workspaceClear(ctx, args[1:])
	default:
		return fmt.Errorf("usage: trybox workspace use [repo] | show | clear")
	}
}

func workspaceUse(ctx context.Context, args []string) error {
	fs, opts := baseFlags("workspace use", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return fmt.Errorf("usage: trybox workspace use [repo]")
	}
	repoInput := opts.Repo
	if len(rest) == 1 {
		repoInput = rest[0]
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	repo, err := resolveRepoForUse(repoInput)
	if err != nil {
		return err
	}
	target, err := targets.Get(targetNameFor(opts, config))
	if err != nil {
		return err
	}
	workspace, err := loadOrCreateWorkspace(store, target, repo)
	if err != nil {
		return err
	}
	applyResourceOverrides(&workspace, target, opts)
	if err := store.SaveWorkspace(workspace); err != nil {
		return err
	}
	config.DefaultTarget = target.Name
	config.DefaultRepoRoot = repo
	config.DefaultWorkspaceID = workspace.ID
	if err := store.SaveConfig(config); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, map[string]any{
			"default_target":       config.DefaultTarget,
			"default_repo_root":    config.DefaultRepoRoot,
			"default_workspace_id": config.DefaultWorkspaceID,
			"workspace":            viewWorkspace(workspace),
		})
	}
	fmt.Printf("default workspace: %s\ntarget:            %s\nrepo:              %s\nvm:                %s\n", workspace.ID, target.Name, repo, workspace.VMName)
	_ = ctx
	return nil
}

func workspaceShow(ctx context.Context, args []string) error {
	fs, opts := baseFlags("workspace show", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	var workspace *workspaceView
	if config.DefaultWorkspaceID != "" {
		if value, err := store.LoadWorkspace(config.DefaultWorkspaceID); err == nil {
			view := viewWorkspace(value)
			workspace = &view
		}
	}
	out := map[string]any{
		"default_target":       config.DefaultTarget,
		"default_repo_root":    config.DefaultRepoRoot,
		"default_workspace_id": config.DefaultWorkspaceID,
		"workspace":            workspace,
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	if config.DefaultWorkspaceID == "" {
		fmt.Println("default workspace: unset")
		return nil
	}
	fmt.Printf("default workspace: %s\ntarget:            %s\nrepo:              %s\n", config.DefaultWorkspaceID, config.DefaultTarget, config.DefaultRepoRoot)
	if workspace != nil {
		fmt.Printf("vm:                %s\n", workspace.VMName)
	}
	_ = ctx
	return nil
}

func workspaceClear(ctx context.Context, args []string) error {
	fs, opts := baseFlags("workspace clear", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	store, _, err := loadStoreConfig()
	if err != nil {
		return err
	}
	if err := store.SaveConfig(state.Config{}); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, map[string]any{"default_workspace_id": ""})
	}
	fmt.Println("default workspace: cleared")
	_ = ctx
	return nil
}

func warmup(ctx context.Context, args []string) error {
	fs, opts := baseFlags("warmup", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, viewWorkspace(workspace))
	}
	fmt.Printf("workspace: %s\ntarget:    %s\nvm:        %s\nip:        %s\nrepo:      %s\n", workspace.ID, workspace.Target, workspace.VMName, workspace.LastKnownIP, workspace.RepoRoot)
	return nil
}

func status(ctx context.Context, args []string) error {
	fs, opts := baseFlags("status", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	exists := b.Exists(ctx, workspace.VMName)
	running := b.IsRunning(ctx, workspace.VMName)
	ip := ""
	if running {
		ip, _ = b.IP(ctx, workspace, 1)
		workspace.LastKnownIP = ip
		_ = store.SaveWorkspace(workspace)
	}
	out := map[string]any{
		"workspace": viewWorkspace(workspace),
		"exists":    exists,
		"running":   running,
		"ip":        ip,
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	fmt.Printf("workspace:   %s\ntarget:      %s\nexists:      %t\nrunning:     %t\nip:          %s\nrepo:        %s\nlast sync:   %s\n",
		workspace.ID, workspace.Target, exists, running, ip, workspace.RepoRoot, workspace.LastSyncAt.Format(time.RFC3339))
	return nil
}

func view(ctx context.Context, args []string) error {
	fs, opts := baseFlags("view", args)
	noOpen := fs.Bool("no-open", false, "print the VNC URL without opening Screen Sharing")
	reuseClient := fs.Bool("reuse-client", false, "reuse any existing Screen Sharing client instead of opening a fresh one")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *noOpen {
		opts.VNC = true
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := b.Create(ctx, target, workspace); err != nil {
		return err
	}
	if !b.IsRunning(ctx, workspace.VMName) {
		if err := b.Start(ctx, target, workspace, backend.StartOptions{Headless: true}); err != nil {
			return err
		}
	}
	if _, err := b.IP(ctx, workspace, 120); err != nil {
		return err
	}
	if err := ensureAutoLogin(ctx, target, workspace, b); err != nil {
		return err
	}
	if b.IsRunning(ctx, workspace.VMName) {
		if err := b.Stop(ctx, workspace); err != nil {
			return err
		}
	}
	if err := b.Start(ctx, target, workspace, backend.StartOptions{VNC: opts.VNC}); err != nil {
		return err
	}
	ip, err := b.IP(ctx, workspace, 120)
	if err != nil {
		return err
	}
	if opts.VNC && *noOpen {
		resetScreenSharingClient(ctx)
	}
	workspace.LastKnownIP = ip
	if err := store.SaveWorkspace(workspace); err != nil {
		return err
	}
	displayURL := vncURL(ip, target.Username, "")
	openURL := displayURL
	if target.Password != "" {
		openURL = vncURL(ip, target.Username, target.Password)
	}
	out := map[string]any{
		"workspace":     viewWorkspace(workspace),
		"display":       viewDisplayName(opts.VNC),
		"client":        viewClientName(opts.VNC, *noOpen),
		"url":           displayURL,
		"username":      target.Username,
		"password_hint": target.Password,
		"fresh_client":  opts.VNC && !*noOpen && !*reuseClient,
		"opened":        !*noOpen,
	}
	if opts.VNC && !*noOpen {
		if !*reuseClient {
			resetScreenSharingClient(ctx)
		}
		if err := exec.CommandContext(ctx, "open", openURL).Start(); err != nil {
			return fmt.Errorf("open Screen Sharing failed: %w", err)
		}
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	fmt.Printf("workspace: %s\ndisplay:   %s\nclient:    %s\n", workspace.ID, viewDisplayName(opts.VNC), viewClientName(opts.VNC, *noOpen))
	if opts.VNC {
		fmt.Printf("url:       %s\nusername:  %s\npassword:  %s\n", displayURL, target.Username, target.Password)
		if *noOpen {
			fmt.Println("open:      skipped")
		} else {
			fmt.Println("open:      Screen Sharing launched")
		}
	} else {
		fmt.Println("open:      Tart native window launched")
	}
	return nil
}

func viewDisplayName(vnc bool) string {
	if vnc {
		return "tart-vnc"
	}
	return "tart-native"
}

func viewClientName(vnc bool, noOpen bool) string {
	if !vnc {
		return "tart"
	}
	if noOpen {
		return "none"
	}
	return "screen-sharing"
}

func resetScreenSharingClient(ctx context.Context) {
	_ = exec.CommandContext(ctx, "osascript", "-e", `tell application "Screen Sharing" to quit`).Run()
	time.Sleep(750 * time.Millisecond)
	_ = exec.CommandContext(ctx, "pkill", "-x", "Screen Sharing").Run()
	time.Sleep(500 * time.Millisecond)
}

func vncURL(host, username, password string) string {
	value := neturl.URL{Scheme: "vnc", Host: host}
	if username != "" && password != "" {
		value.User = neturl.UserPassword(username, password)
	} else if username != "" {
		value.User = neturl.User(username)
	}
	return value.String()
}

func ensureAutoLogin(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend) error {
	if target.Username == "" || target.Password == "" {
		return nil
	}
	expected := "Automatic login user: " + target.Username
	script := strings.Join([]string{
		"set -eu",
		"if sysadminctl -autologin status 2>&1 | grep -F " + shellQuote(expected) + " >/dev/null; then exit 0; fi",
		"printf '%s\\n' " + shellQuote(target.Password) + " | sudo -S sysadminctl -autologin set -userName " + shellQuote(target.Username) + " -password " + shellQuote(target.Password) + " -adminUser " + shellQuote(target.Username) + " -adminPassword " + shellQuote(target.Password) + " >/tmp/trybox-autologin.log 2>&1 || true",
		"sysadminctl -autologin status 2>&1 | grep -F " + shellQuote(expected) + " >/dev/null",
	}, "\n")
	exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", script}, backend.ExecOptions{
		Stdout: io.Discard,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("macOS auto-login setup failed for %s; see /tmp/trybox-autologin.log in the guest", target.Username)
	}
	return nil
}

func stop(ctx context.Context, args []string) error {
	fs, opts := baseFlags("stop", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, workspace, b, _, err := setup(opts)
	if err != nil {
		return err
	}
	return b.Stop(ctx, workspace)
}

func destroy(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("destroy", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: trybox destroy [--json]")
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	if config.DefaultWorkspaceID == "" {
		return fmt.Errorf("no default workspace is configured; run trybox workspace use <repo>")
	}
	workspace, err := store.LoadWorkspace(config.DefaultWorkspaceID)
	if err != nil {
		return fmt.Errorf("load default workspace %q: %w", config.DefaultWorkspaceID, err)
	}
	target, err := targets.Get(workspace.Target)
	if err != nil {
		return err
	}
	b := backendFor(target)
	vmExisted := b.Exists(ctx, workspace.VMName)
	if err := b.Destroy(ctx, workspace); err != nil {
		return err
	}
	out := map[string]any{
		"workspace":               viewWorkspace(workspace),
		"vm_name":                 workspace.VMName,
		"vm_deleted":              vmExisted,
		"host_checkout_untouched": workspace.RepoRoot,
		"workspace_state_kept":    true,
	}
	if *jsonOut {
		return writeJSON(os.Stdout, out)
	}
	if vmExisted {
		fmt.Printf("deleted VM:             %s\n", workspace.VMName)
	} else {
		fmt.Printf("VM already absent:      %s\n", workspace.VMName)
	}
	fmt.Printf("host checkout untouched: %s\nworkspace state kept:    %s\n", workspace.RepoRoot, workspace.ID)
	return nil
}

func syncWorkspace(ctx context.Context, args []string) error {
	fs, opts := baseFlags("sync", args)
	if err := fs.Parse(args); err != nil {
		return err
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
		return err
	}
	result, err := syncWorkspaceState(ctx, target, &workspace, b, store, nil)
	if err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, result)
	}
	printWarnings(result.Warnings)
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
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	run, err := store.NewRun(workspace, command)
	if err != nil {
		return err
	}
	_ = store.AppendEvent(run, "run_created", map[string]any{"command": command})
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
	_, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	repo, err := resolveRepo(opts.Repo, config)
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
	printWarnings(plan.Warnings)
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

func setup(opts *options) (targets.Target, state.Workspace, backend.Backend, state.Store, error) {
	store, config, err := loadStoreConfig()
	if err != nil {
		return targets.Target{}, state.Workspace{}, nil, state.Store{}, err
	}
	target, err := targets.Get(targetNameFor(opts, config))
	if err != nil {
		return targets.Target{}, state.Workspace{}, nil, state.Store{}, err
	}
	repo, err := resolveRepo(opts.Repo, config)
	if err != nil {
		return targets.Target{}, state.Workspace{}, nil, state.Store{}, err
	}
	workspace, err := loadOrCreateWorkspace(store, target, repo)
	if err != nil {
		return targets.Target{}, state.Workspace{}, nil, state.Store{}, err
	}
	applyResourceOverrides(&workspace, target, opts)
	b := backendFor(target)
	return target, workspace, b, store, nil
}

func ensureVM(ctx context.Context, target targets.Target, workspace *state.Workspace, b backend.Backend, store state.Store, opts *options) error {
	if err := b.Create(ctx, target, *workspace); err != nil {
		return err
	}
	if err := b.Start(ctx, target, *workspace, backend.StartOptions{Headless: opts.Headless, VNC: opts.VNC}); err != nil {
		return err
	}
	ip, err := b.IP(ctx, *workspace, 120)
	if err != nil {
		return err
	}
	workspace.LastKnownIP = ip
	return store.SaveWorkspace(*workspace)
}

func syncWorkspaceState(ctx context.Context, target targets.Target, workspaceState *state.Workspace, b backend.Backend, store state.Store, run *state.Run) (syncResult, error) {
	start := time.Now()
	plan, err := workspace.BuildPlan(ctx, workspaceState.RepoRoot, 10)
	if err != nil {
		return syncResult{}, err
	}
	result := syncResult{
		RepoRoot:    workspaceState.RepoRoot,
		RemotePath:  remoteWorkPath(target),
		Fingerprint: plan.Fingerprint,
		FileCount:   plan.FileCount,
		TotalBytes:  plan.TotalBytes,
		Warnings:    plan.Warnings,
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

	if _, err := ensureSSHKey(ctx, target, *workspaceState, b, store); err != nil {
		return result, err
	}
	if matches, err := remoteFingerprintMatches(ctx, target, *workspaceState, b, plan.Fingerprint); err != nil {
		return result, err
	} else if matches {
		result.Skipped = true
		result.Duration = time.Since(start).Round(time.Millisecond).String()
		workspaceState.SyncFingerprint = plan.Fingerprint
		workspaceState.LastSyncAt = time.Now().UTC()
		if err := store.SaveWorkspace(*workspaceState); err != nil {
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

	keyPath := filepath.Join(store.KeyDir(workspaceState.ID), "id_ed25519")
	ip, err := b.IP(ctx, *workspaceState, 120)
	if err != nil {
		return result, err
	}
	sshCmd := fmt.Sprintf("ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR", shellQuote(keyPath))
	remote := fmt.Sprintf("%s@%s:%s/", target.Username, ip, remoteWorkPath(target))
	cmd := exec.CommandContext(ctx, "rsync",
		"-a",
		"--from0",
		"--files-from", manifestPath,
		"--relative",
		"-e", sshCmd,
		"./",
		remote,
	)
	cmd.Dir = workspaceState.RepoRoot
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return result, fmt.Errorf("rsync failed: %w", err)
	}
	if err := writeRemoteFingerprint(ctx, target, *workspaceState, b, plan.Fingerprint); err != nil {
		return result, err
	}
	result.Duration = time.Since(start).Round(time.Millisecond).String()
	workspaceState.SyncFingerprint = plan.Fingerprint
	workspaceState.LastSyncAt = time.Now().UTC()
	if err := store.SaveWorkspace(*workspaceState); err != nil {
		return result, err
	}
	if run != nil {
		_ = store.AppendEvent(*run, "sync_finished", result)
	}
	return result, nil
}

func ensureSSHKey(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend, store state.Store) (string, error) {
	keyDir := store.KeyDir(workspace.ID)
	keyPath := filepath.Join(keyDir, "id_ed25519")
	if err := os.MkdirAll(keyDir, 0o700); err != nil {
		return "", err
	}
	if _, err := os.Stat(keyPath); errors.Is(err, os.ErrNotExist) {
		cmd := exec.CommandContext(ctx, "ssh-keygen", "-t", "ed25519", "-N", "", "-f", keyPath, "-C", "trybox-"+workspace.ID)
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
	exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", install}, backend.ExecOptions{
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

func remoteFingerprintMatches(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend, fingerprint string) (bool, error) {
	check := "test \"$(cat " + shellQuote(remoteFingerprintPath(target)) + " 2>/dev/null)\" = " + shellQuote(fingerprint)
	exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", check}, backend.ExecOptions{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
	if err != nil {
		return false, err
	}
	return exitCode == 0, nil
}

func writeRemoteFingerprint(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend, fingerprint string) error {
	cmd := "mkdir -p " + shellQuote(filepath.Dir(remoteFingerprintPath(target))) + " && printf '%s\\n' " + shellQuote(fingerprint) + " > " + shellQuote(remoteFingerprintPath(target))
	exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", cmd}, backend.ExecOptions{
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
	return filepath.Join("/Users", target.Username, "trybox")
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

func viewWorkspace(workspace state.Workspace) workspaceView {
	return workspaceView{
		ID:              workspace.ID,
		Target:          workspace.Target,
		RepoRoot:        workspace.RepoRoot,
		VMName:          workspace.VMName,
		CPU:             workspace.CPU,
		MemoryMB:        workspace.MemoryMB,
		DiskGB:          workspace.DiskGB,
		CreatedAt:       workspace.CreatedAt,
		UpdatedAt:       workspace.UpdatedAt,
		LastRunLog:      workspace.LastRunLog,
		LastKnownIP:     workspace.LastKnownIP,
		SyncFingerprint: workspace.SyncFingerprint,
		LastSyncAt:      workspace.LastSyncAt,
	}
}

func viewRun(run state.Run) runView {
	return runView{
		ID:          run.ID,
		WorkspaceID: run.WorkspaceID,
		Target:      run.Target,
		RepoRoot:    run.RepoRoot,
		Command:     run.Command,
		StartedAt:   run.StartedAt,
		EndedAt:     run.EndedAt,
		ExitCode:    run.ExitCode,
		StdoutLog:   run.StdoutLog,
		StderrLog:   run.StderrLog,
		EventsLog:   run.EventsLog,
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

func loadStoreConfig() (state.Store, state.Config, error) {
	store, err := state.DefaultStore()
	if err != nil {
		return state.Store{}, state.Config{}, err
	}
	if err := store.Init(); err != nil {
		return state.Store{}, state.Config{}, err
	}
	config, err := store.LoadConfig()
	if err != nil {
		return state.Store{}, state.Config{}, err
	}
	return store, config, nil
}

func targetNameFor(opts *options, config state.Config) string {
	if opts != nil && opts.TargetSet {
		return opts.Target
	}
	if config.DefaultTarget != "" {
		return config.DefaultTarget
	}
	return "macos15-arm64"
}

func loadOrCreateWorkspace(store state.Store, target targets.Target, repo string) (state.Workspace, error) {
	workspaceID := state.WorkspaceID(target.Name, repo)
	workspace, err := store.LoadWorkspace(workspaceID)
	if err == nil {
		return workspace, nil
	}
	return state.Workspace{
		SchemaVersion: 1,
		ID:            workspaceID,
		Target:        target.Name,
		Backend:       target.Backend,
		VMName:        state.WorkspaceVMName(workspaceID),
		RepoRoot:      repo,
		RepoRootHash:  state.RepoRootHash(repo),
		CPU:           target.CPU,
		MemoryMB:      target.MemoryMB,
		DiskGB:        target.DiskGB,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

func applyResourceOverrides(workspace *state.Workspace, target targets.Target, opts *options) {
	if opts.CPU > 0 {
		workspace.CPU = opts.CPU
	}
	if opts.MemoryMB > 0 {
		workspace.MemoryMB = opts.MemoryMB
	}
	if opts.DiskGB > 0 {
		workspace.DiskGB = opts.DiskGB
	}
	if workspace.CPU == 0 {
		workspace.CPU = target.CPU
	}
	if workspace.MemoryMB == 0 {
		workspace.MemoryMB = target.MemoryMB
	}
	if workspace.DiskGB == 0 {
		workspace.DiskGB = target.DiskGB
	}
}

func resolveRepo(repo string, config state.Config) (string, error) {
	if repo != "" {
		return canonicalPath(repo)
	}
	if config.DefaultRepoRoot != "" {
		return canonicalPath(config.DefaultRepoRoot)
	}
	return resolveGitRepo()
}

func resolveRepoForUse(repo string) (string, error) {
	if repo != "" {
		return canonicalPath(repo)
	}
	return resolveGitRepo()
}

func resolveGitRepo() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err == nil {
		return canonicalPath(strings.TrimSpace(string(out)))
	}
	return "", fmt.Errorf("could not detect repo root; pass --repo or run trybox workspace use <repo>")
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs, nil
	}
	return resolved, nil
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

func printWarnings(warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
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
	fmt.Fprint(w, `trybox: clean local Mozilla product debugging workspaces

Usage:
  trybox destroy [--json]
  trybox doctor [--json]
  trybox events <run-id> [--json]
  trybox history [--limit n] [--json]
  trybox logs <run-id>
  trybox run [--target name] [--repo path] [--json] -- <command>
  trybox status [--target name] [--repo path] [--json]
  trybox stop [--target name] [--repo path]
  trybox sync [--target name] [--repo path] [--json]
  trybox sync-plan [--repo path] [--limit n] [--json]
  trybox target list [--json]
  trybox up [--target name] [--repo path] [--cpu n] [--memory-mb n] [--disk-gb n]
  trybox view [--target name] [--repo path] [--vnc] [--no-open] [--reuse-client] [--json]
  trybox workspace clear
  trybox workspace show [--json]
  trybox workspace use [--target name] [--cpu n] [--memory-mb n] [--disk-gb n] [repo]

MVP backend:
  macOS targets via Tart.
`)
}
