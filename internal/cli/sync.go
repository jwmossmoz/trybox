package cli

import (
	"bytes"
	"context"
	"errors"
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
	workspacepkg "github.com/jwmossmoz/trybox/internal/workspace"
)

func syncWorkspace(ctx context.Context, args []string) error {
	fs, opts := commandFlags("sync", flagSpec{Target: true, Repo: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	if err := workspacepkg.ValidateRepoRoot(workspace.RepoRoot); err != nil {
		return err
	}
	var result syncResult
	if err := withWorkspaceLock(ctx, store, workspace.ID, func() error {
		if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
			return err
		}
		var err error
		result, err = syncWorkspaceState(ctx, target, &workspace, b, store, nil)
		return err
	}); err != nil {
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

func syncPlan(ctx context.Context, args []string) error {
	fs, opts := commandFlags("sync-plan", flagSpec{Repo: true, JSON: true})
	limit := fs.Int("limit", 5, "largest files/directories to show")
	if handled, err := parseFlags(fs, args); handled || err != nil {
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
	plan, err := workspacepkg.BuildPlan(ctx, repo, *limit)
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

func syncWorkspaceState(ctx context.Context, target targets.Target, workspaceState *state.Workspace, b backend.Backend, store state.Store, run *state.Run) (syncResult, error) {
	start := time.Now()
	plan, err := workspacepkg.BuildPlan(ctx, workspaceState.RepoRoot, 10)
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
	manifestData := plan.NULManifest()
	remoteManifest, err := readRemoteManifest(ctx, target, *workspaceState, b)
	if err != nil {
		return result, err
	}
	if matches, err := remoteFingerprintMatches(ctx, target, *workspaceState, b, plan.Fingerprint); err != nil {
		return result, err
	} else if matches && bytes.Equal(remoteManifest, manifestData) {
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
	if _, err := manifest.Write(manifestData); err != nil {
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
	staleFiles := staleManifestFiles(remoteManifest, manifestData)
	if err := deleteRemoteFiles(ctx, target, *workspaceState, b, staleFiles); err != nil {
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
	if err := writeRemoteManifest(ctx, target, ip, manifestPath, sshCmd); err != nil {
		return result, err
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

func readRemoteManifest(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend) ([]byte, error) {
	var stdout bytes.Buffer
	cmd := "cat " + shellQuote(remoteManifestPath(target)) + " 2>/dev/null || true"
	exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", cmd}, backend.ExecOptions{
		Stdout: &stdout,
		Stderr: io.Discard,
	})
	if err != nil {
		return nil, err
	}
	if exitCode != 0 {
		return nil, fmt.Errorf("remote manifest read failed with exit code %d", exitCode)
	}
	return stdout.Bytes(), nil
}

func deleteRemoteFiles(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend, files []string) error {
	for len(files) > 0 {
		chunk, rest := deleteChunk(files)
		files = rest
		cmd := "cd " + shellQuote(remoteWorkPath(target)) + " && rm -f -- " + shellJoin(chunk)
		exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", cmd}, backend.ExecOptions{
			Stdout: io.Discard,
			Stderr: os.Stderr,
		})
		if err != nil {
			return err
		}
		if exitCode != 0 {
			return fmt.Errorf("remote stale-file cleanup failed with exit code %d", exitCode)
		}
	}
	return nil
}

func deleteChunk(files []string) ([]string, []string) {
	const maxCommandBytes = 16 * 1024
	if len(files) == 0 {
		return nil, nil
	}
	size := 0
	for i, file := range files {
		size += len(shellQuote(file)) + 1
		if i > 0 && size > maxCommandBytes {
			return files[:i], files[i:]
		}
	}
	return files, nil
}

func writeRemoteManifest(ctx context.Context, target targets.Target, ip string, manifestPath string, sshCmd string) error {
	remote := fmt.Sprintf("%s@%s:%s", target.Username, ip, remoteManifestPath(target))
	cmd := exec.CommandContext(ctx, "rsync", "-a", "-e", sshCmd, manifestPath, remote)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync remote manifest failed: %w", err)
	}
	return nil
}

func staleManifestFiles(previous []byte, current []byte) []string {
	currentFiles := map[string]bool{}
	for _, file := range parseNULManifest(current) {
		currentFiles[file] = true
	}
	stale := []string{}
	for _, file := range parseNULManifest(previous) {
		if !currentFiles[file] && !strings.HasPrefix(file, ".trybox/") {
			stale = append(stale, file)
		}
	}
	sort.Strings(stale)
	return stale
}

func parseNULManifest(data []byte) []string {
	parts := bytes.Split(data, []byte{0})
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if len(part) == 0 {
			continue
		}
		files = append(files, filepath.ToSlash(string(part)))
	}
	return files
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

func remoteManifestPath(target targets.Target) string {
	return filepath.Join(remoteWorkPath(target), ".trybox", "sync-manifest")
}
