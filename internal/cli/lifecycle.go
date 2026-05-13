package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jwmossmoz/trybox/internal/targets"
)

func up(ctx context.Context, args []string) error {
	fs, opts := commandFlags("up", flagSpec{Target: true, Repo: true, JSON: true, Resources: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := withWorkspaceLock(ctx, store, workspace.ID, func() error {
		return ensureVM(ctx, target, &workspace, b, store, opts)
	}); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, viewWorkspace(workspace))
	}
	fmt.Printf("workspace: %s\ntarget:    %s\nvm:        %s\nip:        %s\nrepo:      %s\n", workspace.ID, workspace.Target, workspace.VMName, workspace.LastKnownIP, workspace.RepoRoot)
	return nil
}

func status(ctx context.Context, args []string) error {
	fs, opts := commandFlags("status", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
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

func stop(ctx context.Context, args []string) error {
	fs, opts := commandFlags("stop", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	_, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	return withWorkspaceLock(ctx, store, workspace.ID, func() error {
		wasRunning := b.Exists(ctx, workspace.VMName) && b.IsRunning(ctx, workspace.VMName)
		if err := b.Stop(ctx, workspace); err != nil {
			return err
		}
		if opts.JSON {
			return writeJSON(os.Stdout, map[string]any{
				"workspace":   viewWorkspace(workspace),
				"vm_name":     workspace.VMName,
				"was_running": wasRunning,
				"stopped":     true,
			})
		}
		if wasRunning {
			fmt.Printf("stopped VM: %s\n", workspace.VMName)
		} else {
			fmt.Printf("VM already stopped: %s\n", workspace.VMName)
		}
		return nil
	})
}

func destroy(ctx context.Context, args []string) error {
	fs, opts := commandFlags("destroy", flagSpec{JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return fmt.Errorf("usage: trybox destroy [<workspace-id>] [--json]")
	}
	workspaceID := ""
	if len(rest) == 1 {
		workspaceID = rest[0]
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	workspace, selection, err := workspaceForDestroy(workspaceID, store, config)
	if err != nil {
		return err
	}
	target, err := targets.Get(workspace.Target)
	if err != nil {
		return err
	}
	b := backendFor(target)
	return withWorkspaceLock(ctx, store, workspace.ID, func() error {
		vmExisted := b.Exists(ctx, workspace.VMName)
		if err := b.Destroy(ctx, workspace); err != nil {
			return err
		}
		workspace.LastKnownIP = ""
		workspace.SyncFingerprint = ""
		workspace.LastSyncAt = time.Time{}
		workspace.LastRunLog = ""
		if err := store.SaveWorkspace(workspace); err != nil {
			return fmt.Errorf("clear workspace runtime state: %w", err)
		}
		out := map[string]any{
			"selection":               selection,
			"workspace":               viewWorkspace(workspace),
			"vm_name":                 workspace.VMName,
			"vm_deleted":              vmExisted,
			"host_checkout_untouched": workspace.RepoRoot,
			"workspace_state_kept":    true,
			"runtime_state_cleared":   true,
		}
		if opts.JSON {
			return writeJSON(os.Stdout, out)
		}
		fmt.Printf("selection:              %s\n", selection)
		if vmExisted {
			fmt.Printf("deleted VM:             %s\n", workspace.VMName)
		} else {
			fmt.Printf("VM already absent:      %s\n", workspace.VMName)
		}
		fmt.Printf("host checkout untouched: %s\nworkspace state kept:    %s\n", workspace.RepoRoot, workspace.ID)
		return nil
	})
}
