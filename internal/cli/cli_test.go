package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

func TestWorkspaceForDestroyDefaultAndResolvedSelection(t *testing.T) {
	store := testStore(t)
	target, err := targets.Get("macos15-arm64")
	if err != nil {
		t.Fatal(err)
	}
	repoA := t.TempDir()
	repoB := t.TempDir()
	repoA = canonicalTestPath(t, repoA)
	repoB = canonicalTestPath(t, repoB)
	workspaceA := testWorkspace(target, repoA)
	workspaceB := testWorkspace(target, repoB)
	if err := store.SaveWorkspace(workspaceA); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveWorkspace(workspaceB); err != nil {
		t.Fatal(err)
	}
	config := state.Config{
		DefaultTarget:      target.Name,
		DefaultRepoRoot:    repoA,
		DefaultWorkspaceID: workspaceA.ID,
	}

	got, selection, err := workspaceForDestroy("", store, config)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != workspaceA.ID || selection != "default workspace" {
		t.Fatalf("workspaceForDestroy(default) = %s/%s, want %s/default workspace", got.ID, selection, workspaceA.ID)
	}

	got, selection, err = workspaceForDestroy(workspaceB.ID, store, config)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != workspaceB.ID || selection != "selected workspace" {
		t.Fatalf("workspaceForDestroy(id) = %s/%s, want %s/selected workspace", got.ID, selection, workspaceB.ID)
	}

	if _, _, err := workspaceForDestroy("workspace_does_not_exist", store, config); err == nil {
		t.Fatal("workspaceForDestroy(missing) returned nil error, want failure")
	}
}

func TestStaleManifestFiles(t *testing.T) {
	previous := []byte("keep.txt\x00remove.txt\x00.trybox/sync-fingerprint\x00dir/old.txt\x00")
	current := []byte("keep.txt\x00dir/new.txt\x00")
	got := staleManifestFiles(previous, current)
	want := []string{"dir/old.txt", "remove.txt"}
	if len(got) != len(want) {
		t.Fatalf("staleManifestFiles() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("staleManifestFiles()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestDeleteChunkSplitsLargeCommands(t *testing.T) {
	files := []string{"a", "b", "c"}
	chunk, rest := deleteChunk(files)
	if len(chunk) != len(files) || len(rest) != 0 {
		t.Fatalf("deleteChunk(short) = %v/%v", chunk, rest)
	}

	long := make([]string, 0, 300)
	for i := 0; i < 300; i++ {
		long = append(long, strings.Repeat("x", 100))
	}
	chunk, rest = deleteChunk(long)
	if len(chunk) == 0 || len(rest) == 0 {
		t.Fatalf("deleteChunk(long) = chunk %d rest %d, want split", len(chunk), len(rest))
	}
}

func testStore(t *testing.T) state.Store {
	t.Helper()
	root := t.TempDir()
	store := state.Store{
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

func testWorkspace(target targets.Target, repo string) state.Workspace {
	id := state.WorkspaceID(target.Name, repo)
	return state.Workspace{
		ID:           id,
		Target:       target.Name,
		Backend:      target.Backend,
		VMName:       state.WorkspaceVMName(id),
		RepoRoot:     repo,
		RepoRootHash: state.RepoRootHash(repo),
		CPU:          target.CPU,
		MemoryMB:     target.MemoryMB,
		DiskGB:       target.DiskGB,
		CreatedAt:    time.Now().UTC(),
	}
}

func canonicalTestPath(t *testing.T, path string) string {
	t.Helper()
	canonical, err := canonicalPath(path)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}
