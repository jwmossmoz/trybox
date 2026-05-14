package cli

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jwmossmoz/trybox/internal/state"
)

func TestTargetNameForUsesFlagEnvConfigDefault(t *testing.T) {
	t.Setenv("TRYBOX_TARGET", "macos14-arm64")
	got := targetNameFor(&options{Target: "macos13-arm64", TargetSet: true}, state.Config{DefaultTarget: "macos12-arm64"})
	if got != "macos13-arm64" {
		t.Fatalf("targetNameFor(flag) = %q, want macos13-arm64", got)
	}

	got = targetNameFor(&options{}, state.Config{DefaultTarget: "macos12-arm64"})
	if got != "macos14-arm64" {
		t.Fatalf("targetNameFor(env) = %q, want macos14-arm64", got)
	}

	t.Setenv("TRYBOX_TARGET", "")
	got = targetNameFor(&options{}, state.Config{DefaultTarget: "macos12-arm64"})
	if got != "macos12-arm64" {
		t.Fatalf("targetNameFor(config) = %q, want macos12-arm64", got)
	}

	got = targetNameFor(&options{}, state.Config{})
	if got != "macos15-arm64" {
		t.Fatalf("targetNameFor(default) = %q, want macos15-arm64", got)
	}
}

func TestResolveRepoUsesEnv(t *testing.T) {
	repo := canonicalTestPath(t, t.TempDir())
	t.Setenv("TRYBOX_REPO", repo)
	got, err := resolveRepo("")
	if err != nil {
		t.Fatal(err)
	}
	if got != repo {
		t.Fatalf("resolveRepo(env) = %q, want %q", got, repo)
	}
}

func TestApplyEnvOptionsResources(t *testing.T) {
	t.Setenv("TRYBOX_CPU", "10")
	t.Setenv("TRYBOX_MEMORY_MB", "24576")
	t.Setenv("TRYBOX_DISK_GB", "100")
	opts := &options{Resources: true}
	if err := applyEnvOptions(opts); err != nil {
		t.Fatal(err)
	}
	if opts.CPU != 10 || opts.MemoryMB != 24576 || opts.DiskGB != 100 {
		t.Fatalf("applyEnvOptions() = cpu %d memory %d disk %d", opts.CPU, opts.MemoryMB, opts.DiskGB)
	}
}

func TestApplyEnvOptionsRejectsInvalidResource(t *testing.T) {
	t.Setenv("TRYBOX_CPU", "nope")
	if err := applyEnvOptions(&options{Resources: true}); err == nil {
		t.Fatal("applyEnvOptions(invalid) returned nil error, want failure")
	}
}

func TestViewClientNames(t *testing.T) {
	if got := viewDisplayName(false); got != "tart-native" {
		t.Fatalf("viewDisplayName(native) = %q, want tart-native", got)
	}
	if got := viewDisplayName(true); got != "tart-vnc" {
		t.Fatalf("viewDisplayName(vnc) = %q, want tart-vnc", got)
	}
	if got := viewClientName(false); got != "tart" {
		t.Fatalf("viewClientName(native) = %q, want tart", got)
	}
	if got := viewClientName(true); got != "none" {
		t.Fatalf("viewClientName(vnc) = %q, want none", got)
	}
}

func TestLatestTartVNCURL(t *testing.T) {
	logText := strings.Join([]string{
		"booting",
		"Opening vnc://:old-password@127.0.0.1:5900...",
		"other output",
		"Opening vnc://:new-password@127.0.0.1:52549...",
	}, "\n")
	got := latestTartVNCURL(logText)
	want := "vnc://:new-password@127.0.0.1:52549"
	if got != want {
		t.Fatalf("latestTartVNCURL() = %q, want %q", got, want)
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

func TestRsyncProgressArgsPreferProgress2(t *testing.T) {
	help := "Usage: rsync [OPTION]...\n     --info=FLAGS fine-grained informational verbosity\n"
	got := rsyncProgressArgsFromHelp(help)
	want := []string{"--info=progress2"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("rsyncProgressArgsFromHelp() = %v, want %v", got, want)
	}
}

func TestRsyncProgressArgsFallbackToProgress(t *testing.T) {
	help := "openrsync: protocol version 29\n     [--progress] [--protocol=NUM]\n"
	got := rsyncProgressArgsFromHelp(help)
	want := []string{"--progress"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("rsyncProgressArgsFromHelp() = %v, want %v", got, want)
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

func canonicalTestPath(t *testing.T, path string) string {
	t.Helper()
	canonical, err := canonicalPath(path)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}
