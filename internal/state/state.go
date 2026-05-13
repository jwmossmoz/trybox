package state

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

type Store struct {
	Root          string
	WorkspacesDir string
	RunsDir       string
	LogsDir       string
	KeysDir       string
}

type Config struct {
	SchemaVersion      int       `json:"schema_version"`
	DefaultTarget      string    `json:"default_target,omitempty"`
	DefaultRepoRoot    string    `json:"default_repo_root,omitempty"`
	DefaultWorkspaceID string    `json:"default_workspace_id,omitempty"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type Workspace struct {
	SchemaVersion   int       `json:"schema_version"`
	ID              string    `json:"id"`
	Target          string    `json:"target"`
	Backend         string    `json:"backend"`
	VMName          string    `json:"vm_name"`
	RepoRoot        string    `json:"repo_root"`
	RepoRootHash    string    `json:"repo_root_hash"`
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

type Run struct {
	SchemaVersion int       `json:"schema_version"`
	ID            string    `json:"id"`
	WorkspaceID   string    `json:"workspace_id"`
	Target        string    `json:"target"`
	VMName        string    `json:"vm_name"`
	RepoRoot      string    `json:"repo_root"`
	Command       []string  `json:"command"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at,omitempty"`
	ExitCode      int       `json:"exit_code"`
	StdoutLog     string    `json:"stdout_log"`
	StderrLog     string    `json:"stderr_log"`
	EventsLog     string    `json:"events_log"`
}

type WorkspaceLock struct {
	file *os.File
}

func DefaultStore() (Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Store{}, err
	}
	root := filepath.Join(home, ".trybox")
	return Store{
		Root:          root,
		WorkspacesDir: filepath.Join(root, "workspaces"),
		RunsDir:       filepath.Join(root, "runs"),
		LogsDir:       filepath.Join(root, "logs"),
		KeysDir:       filepath.Join(root, "keys"),
	}, nil
}

func (s Store) Init() error {
	for _, dir := range []string{s.Root, s.WorkspacesDir, s.RunsDir, s.LogsDir, s.KeysDir} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}

func (s Store) WorkspacePath(id string) string {
	return filepath.Join(s.WorkspacesDir, id+".json")
}

func (s Store) WorkspaceLockPath(id string) string {
	return filepath.Join(s.WorkspacesDir, id+".lock")
}

func (s Store) ConfigPath() string {
	return filepath.Join(s.Root, "config.json")
}

func (s Store) LoadConfig() (Config, error) {
	data, err := os.ReadFile(s.ConfigPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Config{}, nil
		}
		return Config{}, err
	}
	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (s Store) SaveConfig(config Config) error {
	config.SchemaVersion = 1
	config.UpdatedAt = time.Now().UTC()
	return writeJSON(s.ConfigPath(), config)
}

func (s Store) SaveWorkspace(workspace Workspace) error {
	workspace.SchemaVersion = 1
	workspace.UpdatedAt = time.Now().UTC()
	if workspace.CreatedAt.IsZero() {
		workspace.CreatedAt = workspace.UpdatedAt
	}
	return writeJSON(s.WorkspacePath(workspace.ID), workspace)
}

func (s Store) LoadWorkspace(id string) (Workspace, error) {
	data, err := os.ReadFile(s.WorkspacePath(id))
	if err != nil {
		return Workspace{}, err
	}
	var workspace Workspace
	if err := json.Unmarshal(data, &workspace); err != nil {
		return Workspace{}, err
	}
	return workspace, nil
}

func (s Store) RemoveWorkspace(id string) error {
	err := os.Remove(s.WorkspacePath(id))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s Store) ListWorkspaces() ([]Workspace, error) {
	entries, err := os.ReadDir(s.WorkspacesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	workspaces := make([]Workspace, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		id := strings.TrimSuffix(name, ".json")
		workspace, err := s.LoadWorkspace(id)
		if err != nil {
			return nil, fmt.Errorf("load workspace %q: %w", id, err)
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, nil
}

func (s Store) RunDir(id string) string {
	return filepath.Join(s.RunsDir, id)
}

func (s Store) KeyDir(workspaceID string) string {
	return filepath.Join(s.KeysDir, workspaceID)
}

func (s Store) NewRun(workspace Workspace, command []string) (Run, error) {
	id := newRunID()
	dir := s.RunDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Run{}, err
	}
	return Run{
		SchemaVersion: 1,
		ID:            id,
		WorkspaceID:   workspace.ID,
		Target:        workspace.Target,
		VMName:        workspace.VMName,
		RepoRoot:      workspace.RepoRoot,
		Command:       command,
		StartedAt:     time.Now().UTC(),
		ExitCode:      -1,
		StdoutLog:     filepath.Join(dir, "stdout.log"),
		StderrLog:     filepath.Join(dir, "stderr.log"),
		EventsLog:     filepath.Join(dir, "events.ndjson"),
	}, nil
}

func (s Store) SaveRun(run Run) error {
	dir := s.RunDir(run.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	run.SchemaVersion = 1
	return writeJSON(filepath.Join(dir, "meta.json"), run)
}

func (s Store) AppendEvent(run Run, event string, payload any) error {
	f, err := os.OpenFile(run.EventsLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	entry := map[string]any{
		"ts":    time.Now().UTC().Format(time.RFC3339Nano),
		"event": event,
	}
	if payload != nil {
		entry["payload"] = payload
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f, string(data))
	return err
}

func (s Store) LockWorkspace(ctx context.Context, id string) (*WorkspaceLock, error) {
	if err := os.MkdirAll(s.WorkspacesDir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(s.WorkspaceLockPath(id), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err == nil {
			return &WorkspaceLock{file: f}, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
			_ = f.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func (l *WorkspaceLock) Unlock() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	if err != nil {
		return err
	}
	return closeErr
}

func WorkspaceID(targetName, repoRoot string) string {
	hash := repoHash(repoRoot)
	return "workspace_" + slug(targetName) + "_" + slug(filepath.Base(repoRoot)) + "_" + hash[:12]
}

func WorkspaceVMName(workspaceID string) string {
	name := strings.TrimPrefix(workspaceID, "workspace_")
	name = strings.ReplaceAll(name, "_", "-")
	if len(name) > 42 {
		parts := strings.Split(name, "-")
		hash := parts[len(parts)-1]
		name = strings.Join(parts[:min(2, len(parts)-1)], "-") + "-" + hash
	}
	return "trybox-ws-" + name
}

func RepoRootHash(repoRoot string) string {
	return repoHash(repoRoot)
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

func newRunID() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "run_" + time.Now().UTC().Format("20060102T150405.000000000Z")
	}
	return "run_" + time.Now().UTC().Format("20060102T150405Z") + "_" + hex.EncodeToString(b[:])
}

func repoHash(repoRoot string) string {
	sum := sha256.Sum256([]byte(repoRoot))
	return hex.EncodeToString(sum[:])
}

func slug(input string) string {
	out := make([]rune, 0, len(input))
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			out = append(out, r)
		case r >= 'A' && r <= 'Z':
			out = append(out, r+'a'-'A')
		case r >= '0' && r <= '9':
			out = append(out, r)
		default:
			if len(out) == 0 || out[len(out)-1] != '-' {
				out = append(out, '-')
			}
		}
	}
	for len(out) > 0 && out[len(out)-1] == '-' {
		out = out[:len(out)-1]
	}
	if len(out) == 0 {
		return "x"
	}
	return string(out)
}
