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
	"sync"
	"time"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
	"github.com/jwmossmoz/trybox/internal/workspace"
)

const syncHeartbeatInterval = 15 * time.Second
const syncRetryHintAfter = 3 * time.Minute

func syncWorkspaceState(ctx context.Context, target targets.Target, workspaceState *state.Workspace, b backend.Backend, store state.Store, run *state.Run, progress io.Writer) (syncResult, error) {
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
		if progress != nil {
			fmt.Fprintf(progress, "sync up to date: %d files, %s\n", plan.FileCount, humanBytes(plan.TotalBytes))
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
	rsyncArgs := []string{
		"-a",
	}
	rsyncArgs = append(rsyncArgs,
		"--from0",
		"--files-from", manifestPath,
		"--relative",
		"-e", sshCmd,
		"./",
		remote,
	)
	if progress != nil {
		fmt.Fprintf(progress, "syncing: %d files, %s -> %s\n", plan.FileCount, humanBytes(plan.TotalBytes), result.RemotePath)
	}
	cmd := exec.CommandContext(ctx, "rsync", rsyncArgs...)
	cmd.Dir = workspaceState.RepoRoot
	progressOut := &newlineTrackingWriter{w: progress}
	cmd.Stdout = progressOut
	cmd.Stderr = progressOut
	if err := runWithSyncHeartbeat(cmd, progressOut, progress, time.Now(), syncRecoveryHint(workspaceState.Target, workspaceState.RepoRoot)); err != nil {
		progressOut.Finish()
		return result, fmt.Errorf("rsync failed: %w", err)
	}
	progressOut.Finish()
	if err := writeRemoteManifest(ctx, target, ip, manifestPath, sshCmd); err != nil {
		return result, err
	}
	if err := writeRemoteFingerprint(ctx, target, *workspaceState, b, plan.Fingerprint); err != nil {
		return result, err
	}
	result.Duration = time.Since(start).Round(time.Millisecond).String()
	if progress != nil {
		fmt.Fprintf(progress, "synced: %d files, %s in %s\n", plan.FileCount, humanBytes(plan.TotalBytes), result.Duration)
	}
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

func runWithSyncHeartbeat(cmd *exec.Cmd, progressOut *newlineTrackingWriter, progress io.Writer, transferStart time.Time, recoveryHint string) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	wait := make(chan error, 1)
	go func() {
		wait <- cmd.Wait()
	}()

	var ticks <-chan time.Time
	var ticker *time.Ticker
	if progress != nil {
		ticker = time.NewTicker(syncHeartbeatInterval)
		defer ticker.Stop()
		ticks = ticker.C
	}

	for {
		select {
		case err := <-wait:
			return err
		case <-ticks:
			progressOut.Finish()
			elapsed := time.Since(transferStart).Round(time.Second)
			if elapsed >= syncRetryHintAfter {
				fmt.Fprintf(progress, "syncing: still copying after %s; if this stalls, cancel and run: %s\n", elapsed, recoveryHint)
			} else {
				fmt.Fprintf(progress, "syncing: still copying after %s\n", elapsed)
			}
		}
	}
}

func syncRecoveryHint(targetName, repoRoot string) string {
	return "trybox destroy --target " + shellQuote(targetName) + " --repo " + shellQuote(repoRoot)
}

type newlineTrackingWriter struct {
	mu    sync.Mutex
	w     io.Writer
	wrote bool
	last  byte
}

func (w *newlineTrackingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(p) > 0 {
		w.wrote = true
		w.last = p[len(p)-1]
	}
	if w.w == nil {
		return len(p), nil
	}
	return w.w.Write(p)
}

func (w *newlineTrackingWriter) Finish() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.wrote || w.last == '\n' || w.w == nil {
		return
	}
	fmt.Fprintln(w.w)
	w.last = '\n'
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
