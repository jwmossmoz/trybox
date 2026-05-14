package cli

import (
	"context"
	"fmt"
	"os"
	"time"
)

func reset(ctx context.Context, args []string) error {
	fs, opts := commandFlags("reset", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: trybox reset [--target name] [--json]")
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	var result syncResult
	vmExisted := false
	if err := withWorkspaceLock(ctx, store, workspace.ID, func() error {
		vmExisted = b.Exists(ctx, workspace.VMName)
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
		if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
			return err
		}
		var err error
		result, err = syncWorkspaceState(ctx, target, &workspace, b, store, nil)
		return err
	}); err != nil {
		return err
	}
	out := map[string]any{
		"workspace":  viewWorkspace(workspace),
		"vm_deleted": vmExisted,
		"sync":       result,
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	if vmExisted {
		fmt.Printf("reset VM: %s\n", workspace.VMName)
	} else {
		fmt.Printf("created VM: %s\n", workspace.VMName)
	}
	printWarnings(result.Warnings)
	fmt.Printf("synced: %d files, %s -> %s (%s)\n", result.FileCount, humanBytes(result.TotalBytes), result.RemotePath, result.Duration)
	return nil
}
