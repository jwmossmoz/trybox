package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jwmossmoz/trybox/internal/ci"
	"github.com/jwmossmoz/trybox/internal/targets"
)

type replayFlags struct {
	Run     bool
	Shell   bool
	RootURL string
}

func taskCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || (len(args) == 1 && isHelp(args[0])) {
		fmt.Fprint(os.Stdout, commandUsage("task"))
		return nil
	}
	opts, replay, rest, err := parseReplayFlags("task", args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: trybox task <task-id> [run|shell] [--root-url URL] [--target name] [--repo path] [--json]")
	}
	plan, err := loadTaskPlan(ctx, replay.RootURL, rest[0])
	if err != nil {
		return err
	}
	if err := applyTargetOverride(opts, &plan); err != nil {
		return err
	}
	return handleReplayPlan(ctx, opts, replay, plan)
}

func tryCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || (len(args) == 1 && isHelp(args[0])) {
		fmt.Fprint(os.Stdout, commandUsage("try"))
		return nil
	}
	opts, replay, rest, err := parseReplayFlags("try", args)
	if err != nil {
		return err
	}
	if len(rest) != 1 && len(rest) != 3 {
		return fmt.Errorf("usage: trybox try <revision-or-url> [task <task-id> [run|shell]] [--root-url URL] [--target name] [--repo path] [--json]")
	}
	revision := revisionFromInput(rest[0])
	source, err := sourceRevisionStatus(opts, rest[0], revision)
	if err != nil {
		return err
	}
	if len(rest) == 1 {
		if opts.JSON {
			return writeJSON(os.Stdout, source)
		}
		printTrySource(source)
		return nil
	}
	if rest[1] != "task" {
		return fmt.Errorf("usage: trybox try <revision-or-url> [task <task-id> [run|shell]] [--root-url URL] [--target name] [--repo path] [--json]")
	}
	plan, err := loadTaskPlan(ctx, replay.RootURL, rest[2])
	if err != nil {
		return err
	}
	if err := applyTargetOverride(opts, &plan); err != nil {
		return err
	}
	plan.Warnings = append(plan.Warnings, sourceWarnings(source)...)
	if (replay.Run || replay.Shell) && !source.Matches {
		return fmt.Errorf("checkout revision %s does not match requested revision %s; check out the requested revision before replaying", source.HeadRevision, source.Revision)
	}
	if opts.JSON && !replay.Run && !replay.Shell {
		return writeJSON(os.Stdout, map[string]any{
			"source": source,
			"task":   plan,
		})
	}
	if !replay.Run && !replay.Shell {
		printTrySource(source)
		fmt.Println()
		printReplayPlan(plan)
		return nil
	}
	return handleReplayPlan(ctx, opts, replay, plan)
}

func handleReplayPlan(ctx context.Context, opts *options, replay replayFlags, plan ci.ReplayPlan) error {
	if replay.Run && replay.Shell {
		return fmt.Errorf("choose only one of run or shell")
	}
	if !replay.Run && !replay.Shell {
		if opts.JSON {
			return writeJSON(os.Stdout, plan)
		}
		printReplayPlan(plan)
		return nil
	}
	if !plan.TargetSupported {
		return fmt.Errorf("task %s is not replayable locally: %s", plan.TaskID, plan.Unsupported)
	}
	if len(plan.Command) == 0 && replay.Run {
		return fmt.Errorf("task %s has no command to run", plan.TaskID)
	}
	if replay.Shell {
		return openTaskShell(ctx, opts, plan)
	}
	command := ci.EnvCommand(plan.Env, plan.Command)
	return runWorkspaceCommand(ctx, opts, command, map[string]any{
		"provider":      plan.Provider,
		"task_id":       plan.TaskID,
		"task_queue_id": plan.TaskQueueID,
	})
}

func loadTaskPlan(ctx context.Context, rootURL, taskID string) (ci.ReplayPlan, error) {
	client := ci.TaskclusterClient{RootURL: rootURL}
	task, err := client.Task(ctx, taskID)
	if err != nil {
		return ci.ReplayPlan{}, err
	}
	return ci.NewReplayPlan(rootURL, taskID, task)
}

func applyTargetOverride(opts *options, plan *ci.ReplayPlan) error {
	if opts.TargetSet {
		if _, err := targets.Get(opts.Target); err != nil {
			return err
		}
		plan.Target = opts.Target
		plan.TargetSupported = true
		plan.Unsupported = ""
		return nil
	}
	if plan.TargetSupported {
		opts.Target = plan.Target
		opts.TargetSet = true
	}
	return nil
}

func parseReplayFlags(command string, args []string) (*options, replayFlags, []string, error) {
	opts := &options{Headless: true}
	replay := replayFlags{RootURL: os.Getenv("TASKCLUSTER_ROOT_URL")}
	rest := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case isHelp(arg):
			return nil, replayFlags{}, nil, fmt.Errorf("help must be the only argument; run trybox help %s", command)
		case arg == "--json":
			opts.JSON = true
		case arg == "--run" || arg == "run":
			replay.Run = true
		case arg == "--shell" || arg == "shell":
			replay.Shell = true
		case arg == "--root-url":
			i++
			if i >= len(args) {
				return nil, replayFlags{}, nil, fmt.Errorf("--root-url requires a value")
			}
			replay.RootURL = args[i]
		case strings.HasPrefix(arg, "--root-url="):
			replay.RootURL = strings.TrimPrefix(arg, "--root-url=")
		case arg == "--target":
			i++
			if i >= len(args) {
				return nil, replayFlags{}, nil, fmt.Errorf("--target requires a value")
			}
			opts.Target = args[i]
			opts.TargetSet = true
		case strings.HasPrefix(arg, "--target="):
			opts.Target = strings.TrimPrefix(arg, "--target=")
			opts.TargetSet = true
		case arg == "--repo":
			i++
			if i >= len(args) {
				return nil, replayFlags{}, nil, fmt.Errorf("--repo requires a value")
			}
			opts.Repo = args[i]
		case strings.HasPrefix(arg, "--repo="):
			opts.Repo = strings.TrimPrefix(arg, "--repo=")
		case strings.HasPrefix(arg, "-"):
			return nil, replayFlags{}, nil, fmt.Errorf("unknown flag %q", arg)
		default:
			rest = append(rest, arg)
		}
	}
	if replay.Run && replay.Shell {
		return nil, replayFlags{}, nil, fmt.Errorf("choose only one of run or shell")
	}
	return opts, replay, rest, nil
}

type trySource struct {
	Input        string `json:"input"`
	Revision     string `json:"revision"`
	RepoRoot     string `json:"repo_root"`
	HeadRevision string `json:"head_revision,omitempty"`
	Matches      bool   `json:"matches"`
}

func sourceRevisionStatus(opts *options, input, revision string) (trySource, error) {
	_, config, err := loadStoreConfig()
	if err != nil {
		return trySource{}, err
	}
	repo, err := resolveRepo(opts.Repo, config)
	if err != nil {
		return trySource{}, err
	}
	head := gitHead(repo)
	return trySource{
		Input:        input,
		Revision:     revision,
		RepoRoot:     repo,
		HeadRevision: head,
		Matches:      revision != "" && head != "" && strings.HasPrefix(head, revision),
	}, nil
}

func gitHead(repo string) string {
	out, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func revisionFromInput(input string) string {
	if parsed, err := url.Parse(input); err == nil && parsed.Scheme != "" {
		for _, key := range []string{"revision", "rev"} {
			if value := parsed.Query().Get(key); value != "" {
				return value
			}
		}
	}
	hex := regexp.MustCompile(`[0-9a-fA-F]{12,40}`)
	if match := hex.FindString(input); match != "" {
		return match
	}
	return input
}

func sourceWarnings(source trySource) []string {
	if source.Matches {
		return nil
	}
	return []string{"host checkout does not match requested revision; plan review only"}
}

func printTrySource(source trySource) {
	fmt.Printf("revision:       %s\nrepo:           %s\nhead revision:  %s\nmatches source: %t\n", source.Revision, source.RepoRoot, source.HeadRevision, source.Matches)
}

func printReplayPlan(plan ci.ReplayPlan) {
	fmt.Printf("task:           %s\n", plan.TaskID)
	if plan.Name != "" {
		fmt.Printf("name:           %s\n", plan.Name)
	}
	fmt.Printf("task queue:     %s\n", plan.TaskQueueID)
	if plan.TargetSupported {
		fmt.Printf("target:         %s\n", plan.Target)
	} else {
		fmt.Printf("target:         unsupported (%s)\n", plan.Unsupported)
	}
	if len(plan.Command) > 0 {
		fmt.Printf("command:        %s\n", shellJoin(plan.Command))
	} else {
		fmt.Println("command:        none")
	}
	fmt.Printf("env vars:       %d\n", len(plan.Env))
	fmt.Printf("dependencies:   %d\n", len(plan.Dependencies))
	fmt.Printf("artifacts:      %d\n", len(plan.Artifacts))
	for _, warning := range plan.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
}

func openTaskShell(ctx context.Context, opts *options, plan ci.ReplayPlan) error {
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	return withWorkspaceLock(ctx, store, workspace.ID, func() error {
		if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
			return err
		}
		if _, err := syncWorkspaceState(ctx, target, &workspace, b, store, nil); err != nil {
			return err
		}
		keyPath, err := ensureSSHKey(ctx, target, workspace, b, store)
		if err != nil {
			return err
		}
		ip, err := b.IP(ctx, workspace, 120)
		if err != nil {
			return err
		}
		workspace.LastKnownIP = ip
		_ = store.SaveWorkspace(workspace)
		remote := "cd " + shellQuote(remoteWorkPath(target))
		if prelude := ci.ShellPrelude(plan.Env); prelude != "" {
			remote += " && " + prelude
		}
		remote += " && exec ${SHELL:-/bin/sh} -l"
		sshArgs := []string{
			"-i", keyPath,
			"-o", "IdentitiesOnly=yes",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
			target.Username + "@" + ip,
			remote,
		}
		cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitError{Code: exitErr.ExitCode()}
			}
			return fmt.Errorf("ssh %s: %w", filepath.Base(keyPath), err)
		}
		return nil
	})
}
