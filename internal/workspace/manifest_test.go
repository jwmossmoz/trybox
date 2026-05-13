package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPlanIncludesGitWorkingSetAndMetadata(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}
	repo := t.TempDir()
	runGit(t, repo, "init")
	writeFile(t, filepath.Join(repo, "tracked.txt"), "tracked")
	runGit(t, repo, "add", "tracked.txt")
	writeFile(t, filepath.Join(repo, "untracked.txt"), "untracked")
	writeFile(t, filepath.Join(repo, "node_modules", "ignored.js"), "ignored")

	plan, err := BuildPlan(context.Background(), repo, 10)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, file := range plan.Files() {
		files[file.Path] = true
	}
	for _, path := range []string{"tracked.txt", "untracked.txt", ".git/HEAD"} {
		if !files[path] {
			t.Fatalf("BuildPlan files missing %q; files=%v", path, files)
		}
	}
	if files["node_modules/ignored.js"] {
		t.Fatalf("BuildPlan included default-excluded node_modules file")
	}
	if !contains(plan.Excluded, "node_modules/ignored.js") {
		t.Fatalf("BuildPlan excluded = %v, want node_modules/ignored.js", plan.Excluded)
	}
	if !strings.Contains(string(plan.NULManifest()), "tracked.txt\x00") {
		t.Fatalf("NULManifest() does not contain tracked.txt entry")
	}
}

func TestBuildPlanRejectsGitfileWorktree(t *testing.T) {
	repo := t.TempDir()
	writeFile(t, filepath.Join(repo, ".git"), "gitdir: /Users/example/source/.git/worktrees/feature\n")

	_, err := BuildPlan(context.Background(), repo, 10)
	if err == nil {
		t.Fatal("BuildPlan() returned nil error, want git worktree rejection")
	}
	message := err.Error()
	for _, want := range []string{"git worktree", "/Users/example/source/.git/worktrees/feature", "git clone --no-local --no-hardlinks"} {
		if !strings.Contains(message, want) {
			t.Fatalf("BuildPlan() error = %q, want substring %q", message, want)
		}
	}
}

func TestIsExcluded(t *testing.T) {
	excludes := []string{"node_modules/", "*.log", "build/cache"}
	cases := map[string]bool{
		"node_modules/pkg/index.js": true,
		"node_modules":              true,
		"debug.log":                 true,
		"build/cache/file":          true,
		"build/output/file":         false,
		"src/app.go":                false,
	}
	for path, want := range cases {
		if got := isExcluded(path, excludes); got != want {
			t.Fatalf("isExcluded(%q) = %t, want %t", path, got, want)
		}
	}
}

func TestLargestDirsStableOrdering(t *testing.T) {
	files := []File{
		{Path: "b/file", Size: 10},
		{Path: "a/file", Size: 10},
		{Path: "a/other", Size: 5},
		{Path: "top", Size: 20},
	}
	got := largestDirs(files, 3)
	want := []File{
		{Path: ".", Size: 20},
		{Path: "a", Size: 15},
		{Path: "b", Size: 10},
	}
	for i := range want {
		if got[i].Path != want[i].Path || got[i].Size != want[i].Size {
			t.Fatalf("largestDirs()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func runGit(t *testing.T, repo string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
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

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
