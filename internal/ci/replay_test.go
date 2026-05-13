package ci

import (
	"encoding/json"
	"testing"
)

func TestCommandArgs(t *testing.T) {
	args, err := CommandArgs(json.RawMessage(`["bash","-lc","echo ok"]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || args[0] != "bash" {
		t.Fatalf("CommandArgs(array) = %v", args)
	}
	args, err = CommandArgs(json.RawMessage(`"echo ok"`))
	if err != nil {
		t.Fatal(err)
	}
	if len(args) != 3 || args[0] != "sh" || args[2] != "echo ok" {
		t.Fatalf("CommandArgs(string) = %v", args)
	}
}

func TestTargetForTask(t *testing.T) {
	target, ok, reason := TargetForTask(TaskDefinition{TaskQueueID: "proj/macosx1500-aarch64"})
	if !ok || target != "macos15-arm64" || reason != "" {
		t.Fatalf("TargetForTask(macos arm) = %q/%t/%q", target, ok, reason)
	}
	target, ok, reason = TargetForTask(TaskDefinition{TaskQueueID: "proj/macosx1400-x86_64"})
	if !ok || target != "macos14-x64-rosetta" || reason != "" {
		t.Fatalf("TargetForTask(macos x64) = %q/%t/%q", target, ok, reason)
	}
	_, ok, reason = TargetForTask(TaskDefinition{TaskQueueID: "proj/win11-64"})
	if ok || reason == "" {
		t.Fatalf("TargetForTask(windows) ok=%t reason=%q, want unsupported reason", ok, reason)
	}
}

func TestEnvCommandSortsEnvironment(t *testing.T) {
	got := EnvCommand(map[string]string{"B": "2", "A": "1"}, []string{"echo", "ok"})
	want := []string{"env", "A=1", "B=2", "echo", "ok"}
	if len(got) != len(want) {
		t.Fatalf("EnvCommand() = %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("EnvCommand()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
