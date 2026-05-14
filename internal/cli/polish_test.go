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
