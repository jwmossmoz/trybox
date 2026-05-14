package cli

import (
	"strings"
	"testing"

	"github.com/jwmossmoz/trybox/internal/targets"
)

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
