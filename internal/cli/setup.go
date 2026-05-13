package cli

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

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

func withWorkspaceLock(ctx context.Context, store state.Store, workspaceID string, fn func() error) error {
	lock, err := store.LockWorkspace(ctx, workspaceID)
	if err != nil {
		return fmt.Errorf("lock workspace %q: %w", workspaceID, err)
	}
	defer lock.Unlock()
	return fn()
}

func workspaceForDestroy(workspaceID string, store state.Store, config state.Config) (state.Workspace, string, error) {
	if workspaceID != "" {
		workspace, err := store.LoadWorkspace(workspaceID)
		if err != nil {
			return state.Workspace{}, "", fmt.Errorf("load workspace %q: %w (see trybox workspace list)", workspaceID, err)
		}
		return workspace, "selected workspace", nil
	}
	if config.DefaultWorkspaceID == "" {
		return state.Workspace{}, "", fmt.Errorf("no default workspace is configured; pass a workspace id (see trybox workspace list) or run trybox workspace use")
	}
	workspace, err := store.LoadWorkspace(config.DefaultWorkspaceID)
	if err != nil {
		return state.Workspace{}, "", fmt.Errorf("load default workspace %q: %w (run trybox workspace list to see known workspaces, or trybox workspace unset to clear the default)", config.DefaultWorkspaceID, err)
	}
	return workspace, "default workspace", nil
}

func ensureVM(ctx context.Context, target targets.Target, workspace *state.Workspace, b backend.Backend, store state.Store, opts *options) error {
	if resourceOverridesRequested(opts) && b.Exists(ctx, workspace.VMName) {
		return fmt.Errorf("resource changes require destroying existing workspace VM %q first; run: trybox destroy %s", workspace.VMName, workspace.ID)
	}
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

func resourceOverridesRequested(opts *options) bool {
	return opts != nil && (opts.CPU > 0 || opts.MemoryMB > 0 || opts.DiskGB > 0)
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
