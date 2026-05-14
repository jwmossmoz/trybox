package cli

import (
	"strings"
	"testing"

	"github.com/jwmossmoz/trybox/internal/targets"
)

func TestRsyncVersionHintFromOutput(t *testing.T) {
	got := rsyncVersionHintFromOutput("rsync  version 2.6.9  protocol version 29\n")
	if !strings.Contains(got, "brew install rsync") {
		t.Fatalf("rsyncVersionHintFromOutput() = %q, want brew hint", got)
	}
	if got := rsyncVersionHintFromOutput("rsync  version 3.2.7  protocol version 31\n"); got != "" {
		t.Fatalf("rsyncVersionHintFromOutput(modern) = %q, want empty", got)
	}
}

func TestTargetCloneCommand(t *testing.T) {
	target, err := targets.Get("macos15-arm64")
	if err != nil {
		t.Fatal(err)
	}
	got := targetCloneCommand(target)
	if !strings.Contains(got, target.SourceImage) || !strings.Contains(got, target.ImageName) {
		t.Fatalf("targetCloneCommand() = %q, want source and image", got)
	}
}

func TestApplyResourceProfileAndExplicitOverride(t *testing.T) {
	target, err := targets.Get("macos15-arm64")
	if err != nil {
		t.Fatal(err)
	}
	workspace := testWorkspace(target, canonicalTestPath(t, t.TempDir()))
	if err := applyResourceOverrides(&workspace, target, &options{Profile: "test", CPU: 6}); err != nil {
		t.Fatal(err)
	}
	if workspace.CPU != 6 || workspace.MemoryMB != 8192 || workspace.DiskGB != 80 {
		t.Fatalf("profile result = cpu %d memory %d disk %d, want 6/8192/80", workspace.CPU, workspace.MemoryMB, workspace.DiskGB)
	}
	if err := applyResourceOverrides(&workspace, target, &options{Profile: "unknown"}); err == nil {
		t.Fatal("applyResourceOverrides(unknown profile) returned nil error, want failure")
	}
}

func TestCleanRemoteDestination(t *testing.T) {
	if got := cleanRemoteDestination("/Users/admin/trybox", "artifacts/build.dmg"); got != "/Users/admin/trybox/artifacts/build.dmg" {
		t.Fatalf("cleanRemoteDestination(relative) = %q", got)
	}
	if got := cleanRemoteDestination("/Users/admin/trybox", "/tmp/build.dmg"); got != "/tmp/build.dmg" {
		t.Fatalf("cleanRemoteDestination(absolute) = %q", got)
	}
}
