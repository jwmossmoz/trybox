package cli

import (
	"context"
	"fmt"
	"os"
	"time"
)

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
		"vm":      viewWorkspace(workspace),
		"exists":  exists,
		"running": running,
		"ip":      ip,
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	lastSync := "never"
	if !workspace.LastSyncAt.IsZero() {
		lastSync = workspace.LastSyncAt.Format(time.RFC3339)
	}
	fmt.Printf("vm:        %s\ntarget:    %s\nexists:    %t\nrunning:   %t\nip:        %s\nrepo:      %s\nlast sync: %s\n",
		workspace.VMName, workspace.Target, exists, running, ip, workspace.RepoRoot, lastSync)
	return nil
}

func destroy(ctx context.Context, args []string) error {
	fs, opts := commandFlags("destroy", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: trybox destroy [--target name] [--repo path] [--json]")
	}
	_, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
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
			"vm":                      viewWorkspace(workspace),
			"vm_name":                 workspace.VMName,
			"vm_deleted":              vmExisted,
			"host_checkout_untouched": workspace.RepoRoot,
			"state_kept":              true,
			"runtime_state_cleared":   true,
		}
		if opts.JSON {
			return writeJSON(os.Stdout, out)
		}
		if vmExisted {
			fmt.Printf("deleted VM:              %s\n", workspace.VMName)
		} else {
			fmt.Printf("VM already absent:       %s\n", workspace.VMName)
		}
		fmt.Printf("host checkout untouched: %s\nstate kept:              %s\n", workspace.RepoRoot, workspace.ID)
		return nil
	})
}
