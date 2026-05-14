package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
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

func TestParseLogsArgsAllowsFlagsBeforeOrAfterRunID(t *testing.T) {
	runID, opts, err := parseLogsArgs([]string{"run_1", "--follow", "--from-end"})
	if err != nil {
		t.Fatal(err)
	}
	if runID != "run_1" || !opts.Follow || !opts.FromEnd {
		t.Fatalf("parseLogsArgs(after) = %q %+v, want follow/from-end", runID, opts)
	}
	runID, opts, err = parseLogsArgs([]string{"-f", "run_2"})
	if err != nil {
		t.Fatal(err)
	}
	if runID != "run_2" || !opts.Follow || opts.FromEnd {
		t.Fatalf("parseLogsArgs(before) = %q %+v, want follow only", runID, opts)
	}
}

func TestFollowRunLogsCompletedReturnsExitCode(t *testing.T) {
	store := testStore(t)
	run := testRun(t, store, "run_follow_completed")
	run.EndedAt = time.Now().UTC()
	run.ExitCode = 7
	if err := store.SaveRun(run); err != nil {
		t.Fatal(err)
	}
	writeFile(t, run.StdoutLog, "stdout\n")
	writeFile(t, run.StderrLog, "stderr\n")

	var out bytes.Buffer
	err := followRunLogs(context.Background(), &out, store, run.ID, logFollowOptions{})
	var exit exitError
	if !errors.As(err, &exit) || exit.Code != 7 {
		t.Fatalf("followRunLogs() error = %v, want exit 7", err)
	}
	if got, want := out.String(), "stdout\nstderr\n"; got != want {
		t.Fatalf("followRunLogs() output = %q, want %q", got, want)
	}
}

func TestFollowRunLogsUsesCommandFinishedEvent(t *testing.T) {
	store := testStore(t)
	run := testRun(t, store, "run_follow_event")
	if err := store.SaveRun(run); err != nil {
		t.Fatal(err)
	}
	writeFile(t, run.StdoutLog, "ready\n")
	if err := store.AppendEvent(run, "command_finished", map[string]any{"exit_code": 3}); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := followRunLogs(context.Background(), &out, store, run.ID, logFollowOptions{})
	var exit exitError
	if !errors.As(err, &exit) || exit.Code != 3 {
		t.Fatalf("followRunLogs() error = %v, want exit 3", err)
	}
	if got, want := out.String(), "ready\n"; got != want {
		t.Fatalf("followRunLogs() output = %q, want %q", got, want)
	}
}

func TestCopyAvailableLogFromEnd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdout.log")
	writeFile(t, path, "old\n")
	offset := int64(-1)
	var out bytes.Buffer
	if err := copyAvailableLog(&out, path, &offset, true); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 {
		t.Fatalf("copyAvailableLog(fromEnd) output = %q, want empty", out.String())
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("new\n"); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := copyAvailableLog(&out, path, &offset, true); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "new\n"; got != want {
		t.Fatalf("copyAvailableLog() output = %q, want %q", got, want)
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

func testRun(t *testing.T, store state.Store, id string) state.Run {
	t.Helper()
	dir := store.RunDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	return state.Run{
		SchemaVersion: 1,
		ID:            id,
		WorkspaceID:   "workspace_test",
		Target:        "macos15-arm64",
		VMName:        "trybox-ws-test",
		RepoRoot:      t.TempDir(),
		Command:       []string{"echo", "test"},
		StartedAt:     time.Now().UTC(),
		ExitCode:      -1,
		StdoutLog:     filepath.Join(dir, "stdout.log"),
		StderrLog:     filepath.Join(dir, "stderr.log"),
		EventsLog:     filepath.Join(dir, "events.ndjson"),
	}
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

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
