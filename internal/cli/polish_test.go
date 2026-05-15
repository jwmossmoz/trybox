package cli

import (
	"strings"
	"testing"

	"github.com/jwmossmoz/trybox/internal/targets"
)

func TestTargetBootstrapCommand(t *testing.T) {
	target, err := targets.Get("macos15-arm64")
	if err != nil {
		t.Fatal(err)
	}
	got := targetBootstrapCommand(target)
	if !strings.Contains(got, "trybox bootstrap") || !strings.Contains(got, target.Name) {
		t.Fatalf("targetBootstrapCommand() = %q, want trybox bootstrap and target", got)
	}
}
