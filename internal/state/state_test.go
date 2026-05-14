package state

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWorkspaceIDAndVMName(t *testing.T) {
	repo := filepath.Join(string(os.PathSeparator), "tmp", "My Repo")
	id := WorkspaceID("macOS 15/ARM64", repo)
	want := "workspace_macos-15-arm64_my-repo_" + repoHash(repo)[:12]
	if id != want {
		t.Fatalf("WorkspaceID() = %q, want %q", id, want)
	}

	vmName := WorkspaceVMName(id)
	if vmName == "" || len(vmName) > len("trybox-ws-")+42 {
		t.Fatalf("WorkspaceVMName() = %q, want non-empty shortened name", vmName)
	}
}

func TestStoreSaveLoad(t *testing.T) {
	store := testStore(t)

	config := Config{
		DefaultTarget: "macos15-arm64",
	}
	if err := store.SaveConfig(config); err != nil {
		t.Fatal(err)
	}
	loadedConfig, err := store.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if loadedConfig.SchemaVersion != 1 || loadedConfig.DefaultTarget != config.DefaultTarget || loadedConfig.UpdatedAt.IsZero() {
		t.Fatalf("LoadConfig() = %+v", loadedConfig)
	}

	workspace := Workspace{
		ID:       "workspace_macos15-arm64_repo_abc",
		Target:   "macos15-arm64",
		Backend:  "tart",
		VMName:   "trybox-ws-repo-abc",
		RepoRoot: "/tmp/repo",
	}
	if err := store.SaveWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	loadedWorkspace, err := store.LoadWorkspace(workspace.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedWorkspace.SchemaVersion != 1 || loadedWorkspace.CreatedAt.IsZero() || loadedWorkspace.UpdatedAt.IsZero() {
		t.Fatalf("LoadWorkspace() = %+v", loadedWorkspace)
	}
}

func TestWorkspaceLockBlocksConcurrentLock(t *testing.T) {
	store := testStore(t)
	first, err := store.LockWorkspace(context.Background(), "workspace_test")
	if err != nil {
		t.Fatal(err)
	}
	defer first.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := store.LockWorkspace(ctx, "workspace_test"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("second LockWorkspace() error = %v, want deadline exceeded", err)
	}

	if err := first.Unlock(); err != nil {
		t.Fatal(err)
	}
	second, err := store.LockWorkspace(context.Background(), "workspace_test")
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Unlock(); err != nil {
		t.Fatal(err)
	}
}

func testStore(t *testing.T) Store {
	t.Helper()
	root := t.TempDir()
	store := Store{
		Root:          root,
		WorkspacesDir: filepath.Join(root, "workspaces"),
		RunsDir:       filepath.Join(root, "runs"),
		LogsDir:       filepath.Join(root, "logs"),
		KeysDir:       filepath.Join(root, "keys"),
	}
	if err := store.Init(); err != nil {
		t.Fatal(err)
	}
	return store
}
