package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
)

type snapshotMeta struct {
	SchemaVersion   int       `json:"schema_version"`
	Name            string    `json:"name"`
	WorkspaceID     string    `json:"workspace_id"`
	Target          string    `json:"target"`
	RepoRoot        string    `json:"repo_root"`
	VMName          string    `json:"vm_name"`
	SnapshotVMName  string    `json:"snapshot_vm_name"`
	SyncFingerprint string    `json:"sync_fingerprint,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	NominalBytes    int64     `json:"nominal_bytes,omitempty"`
	DiskBytes       int64     `json:"disk_bytes,omitempty"`
}

func snapshotCommand(ctx context.Context, args []string) error {
	if len(args) == 0 || isHelp(args[0]) {
		fmt.Fprint(os.Stdout, commandUsage("snapshot"))
		_ = ctx
		return nil
	}
	if len(args) > 1 && isHelp(args[1]) {
		return printCommandHelp([]string{"snapshot", args[0]})
	}
	switch args[0] {
	case "save":
		return snapshotSave(ctx, args[1:])
	case "list":
		return snapshotList(ctx, args[1:])
	case "restore":
		return snapshotRestore(ctx, args[1:])
	case "delete":
		return snapshotDelete(ctx, args[1:])
	default:
		return fmt.Errorf("unknown snapshot subcommand %q; run trybox help snapshot for usage", args[0])
	}
}

func snapshotSave(ctx context.Context, args []string) error {
	fs, opts := commandFlags("snapshot save", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 1 {
		return fmt.Errorf("usage: trybox snapshot save <name> [--target name] [--repo path] [--json]")
	}
	name := fs.Args()[0]
	if err := validateSnapshotName(name); err != nil {
		return err
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	var meta snapshotMeta
	if err := withWorkspaceLock(ctx, store, workspace.ID, func() error {
		if _, err := loadSnapshot(store, workspace.ID, name); err == nil {
			return fmt.Errorf("snapshot %q already exists", name)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		meta = snapshotMeta{
			SchemaVersion:   1,
			Name:            name,
			WorkspaceID:     workspace.ID,
			Target:          workspace.Target,
			RepoRoot:        workspace.RepoRoot,
			VMName:          workspace.VMName,
			SnapshotVMName:  snapshotVMName(workspace, name),
			SyncFingerprint: workspace.SyncFingerprint,
			CreatedAt:       time.Now().UTC(),
		}
		if err := b.SnapshotSave(ctx, target, workspace, meta.SnapshotVMName); err != nil {
			return err
		}
		if size, err := b.SnapshotSize(ctx, meta.SnapshotVMName); err == nil {
			meta.NominalBytes = size.NominalBytes
			meta.DiskBytes = size.DiskBytes
		}
		return saveSnapshot(store, meta)
	}); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, meta)
	}
	fmt.Printf("snapshot saved: %s (%s on disk, %s nominal)\n", meta.Name, humanBytes(meta.DiskBytes), humanBytes(meta.NominalBytes))
	return nil
}

func snapshotList(ctx context.Context, args []string) error {
	fs, opts := commandFlags("snapshot list", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: trybox snapshot list [--target name] [--repo path] [--json]")
	}
	_, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	snapshots, err := listSnapshots(store, workspace.ID)
	if err != nil {
		return err
	}
	for i := range snapshots {
		if size, err := b.SnapshotSize(ctx, snapshots[i].SnapshotVMName); err == nil {
			snapshots[i].NominalBytes = size.NominalBytes
			snapshots[i].DiskBytes = size.DiskBytes
			_ = saveSnapshot(store, snapshots[i])
		}
	}
	if opts.JSON {
		return writeJSON(os.Stdout, snapshots)
	}
	if len(snapshots) == 0 {
		fmt.Println("no snapshots")
		return nil
	}
	for _, snapshot := range snapshots {
		fmt.Printf("%-24s disk=%-8s nominal=%-8s created=%s vm=%s\n",
			snapshot.Name,
			humanBytes(snapshot.DiskBytes),
			humanBytes(snapshot.NominalBytes),
			snapshot.CreatedAt.Format(time.RFC3339),
			snapshot.SnapshotVMName,
		)
	}
	return nil
}

func snapshotRestore(ctx context.Context, args []string) error {
	fs, opts := commandFlags("snapshot restore", flagSpec{Target: true, Repo: true, JSON: true})
	display := fs.Bool("display", false, "restart with a display instead of headless")
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 1 {
		return fmt.Errorf("usage: trybox snapshot restore <name> [--display] [--target name] [--repo path] [--json]")
	}
	name := fs.Args()[0]
	if err := validateSnapshotName(name); err != nil {
		return err
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	var meta snapshotMeta
	warnings := []string{}
	if err := withWorkspaceLock(ctx, store, workspace.ID, func() error {
		var err error
		meta, err = loadSnapshot(store, workspace.ID, name)
		if err != nil {
			return err
		}
		if meta.SyncFingerprint != "" && workspace.SyncFingerprint != "" && meta.SyncFingerprint != workspace.SyncFingerprint {
			warnings = append(warnings, "workspace sync fingerprint differs from the restored snapshot; run trybox sync if host sources should be reapplied")
		}
		startOpts := backend.StartOptions{Headless: !*display}
		if err := b.SnapshotRestore(ctx, target, workspace, meta.SnapshotVMName, startOpts); err != nil {
			return err
		}
		ip, err := b.IP(ctx, workspace, 120)
		if err != nil {
			return err
		}
		workspace.LastKnownIP = ip
		workspace.SyncFingerprint = meta.SyncFingerprint
		workspace.LastSyncAt = meta.CreatedAt
		return store.SaveWorkspace(workspace)
	}); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, map[string]any{
			"snapshot": meta,
			"restored": true,
			"display":  *display,
			"warnings": warnings,
		})
	}
	printWarnings(warnings)
	mode := "headless"
	if *display {
		mode = "display"
	}
	fmt.Printf("snapshot restored: %s (%s)\n", meta.Name, mode)
	return nil
}

func snapshotDelete(ctx context.Context, args []string) error {
	fs, opts := commandFlags("snapshot delete", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 1 {
		return fmt.Errorf("usage: trybox snapshot delete <name> [--target name] [--repo path] [--json]")
	}
	name := fs.Args()[0]
	if err := validateSnapshotName(name); err != nil {
		return err
	}
	_, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	var meta snapshotMeta
	if err := withWorkspaceLock(ctx, store, workspace.ID, func() error {
		var err error
		meta, err = loadSnapshot(store, workspace.ID, name)
		if err != nil {
			return err
		}
		if err := b.SnapshotDelete(ctx, meta.SnapshotVMName); err != nil {
			return err
		}
		return os.Remove(snapshotPath(store, workspace.ID, name))
	}); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, map[string]any{"snapshot": meta, "deleted": true})
	}
	fmt.Printf("snapshot deleted: %s\n", meta.Name)
	return nil
}

func validateSnapshotName(name string) error {
	if name == "" {
		return fmt.Errorf("snapshot name cannot be empty")
	}
	lastHyphen := false
	for i, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			lastHyphen = false
		case r >= '0' && r <= '9':
			lastHyphen = false
		case r == '-':
			if i == 0 || lastHyphen {
				return fmt.Errorf("snapshot name %q must be kebab-case", name)
			}
			lastHyphen = true
		default:
			return fmt.Errorf("snapshot name %q must be kebab-case", name)
		}
	}
	if lastHyphen {
		return fmt.Errorf("snapshot name %q must be kebab-case", name)
	}
	return nil
}

func snapshotVMName(workspace state.Workspace, name string) string {
	return workspace.VMName + ".snapshot." + name
}

func snapshotDir(store state.Store, workspaceID string) string {
	return filepath.Join(store.WorkspacesDir, workspaceID, "snapshots")
}

func snapshotPath(store state.Store, workspaceID, name string) string {
	return filepath.Join(snapshotDir(store, workspaceID), name+".json")
}

func saveSnapshot(store state.Store, meta snapshotMeta) error {
	if err := os.MkdirAll(snapshotDir(store, meta.WorkspaceID), 0o700); err != nil {
		return err
	}
	meta.SchemaVersion = 1
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(snapshotPath(store, meta.WorkspaceID, meta.Name), append(data, '\n'), 0o600)
}

func loadSnapshot(store state.Store, workspaceID, name string) (snapshotMeta, error) {
	data, err := os.ReadFile(snapshotPath(store, workspaceID, name))
	if err != nil {
		return snapshotMeta{}, err
	}
	var meta snapshotMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return snapshotMeta{}, err
	}
	return meta, nil
}

func listSnapshots(store state.Store, workspaceID string) ([]snapshotMeta, error) {
	entries, err := os.ReadDir(snapshotDir(store, workspaceID))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []snapshotMeta{}, nil
		}
		return nil, err
	}
	snapshots := []snapshotMeta{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(snapshotDir(store, workspaceID), entry.Name()))
		if err != nil {
			return nil, err
		}
		var meta snapshotMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, meta)
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots, nil
}
