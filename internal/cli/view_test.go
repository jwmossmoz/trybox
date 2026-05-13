package cli

import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

func TestEnsureAutoLoginReportsConfigured(t *testing.T) {
	target, workspace := testViewTargetWorkspace(t)
	b := &recordingBackend{execStdout: "configured\n"}

	changed, err := ensureAutoLogin(context.Background(), target, workspace, b)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("ensureAutoLogin() changed = false, want true")
	}
}

func TestEnsureAutoLoginReportsAlreadyConfigured(t *testing.T) {
	target, workspace := testViewTargetWorkspace(t)
	b := &recordingBackend{execStdout: "already\n"}

	changed, err := ensureAutoLogin(context.Background(), target, workspace, b)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("ensureAutoLogin() changed = true, want false")
	}
}

func TestCompleteAutoLoginBootCycleRestartsHeadlessAfterConfig(t *testing.T) {
	target, workspace := testViewTargetWorkspace(t)
	b := &recordingBackend{execStdout: "configured\n", running: true}

	if err := completeAutoLoginBootCycle(context.Background(), target, workspace, b); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec", "stop", "start headless=true vnc=false", "ip"}
	if fmt.Sprint(b.actions) != fmt.Sprint(want) {
		t.Fatalf("actions = %v, want %v", b.actions, want)
	}
}

func TestCompleteAutoLoginBootCycleSkipsRestartWhenAlreadyConfigured(t *testing.T) {
	target, workspace := testViewTargetWorkspace(t)
	b := &recordingBackend{execStdout: "already\n", running: true}

	if err := completeAutoLoginBootCycle(context.Background(), target, workspace, b); err != nil {
		t.Fatal(err)
	}
	want := []string{"exec"}
	if fmt.Sprint(b.actions) != fmt.Sprint(want) {
		t.Fatalf("actions = %v, want %v", b.actions, want)
	}
}

func testViewTargetWorkspace(t *testing.T) (targets.Target, state.Workspace) {
	t.Helper()
	target, err := targets.Get("macos15-arm64")
	if err != nil {
		t.Fatal(err)
	}
	return target, testWorkspace(target, t.TempDir())
}

type recordingBackend struct {
	execStdout string
	execExit   int
	running    bool
	actions    []string
}

func (b *recordingBackend) Name() string {
	return "recording"
}

func (b *recordingBackend) Doctor(context.Context, targets.Target) []backend.Check {
	return nil
}

func (b *recordingBackend) Exists(context.Context, string) bool {
	return true
}

func (b *recordingBackend) IsRunning(context.Context, string) bool {
	return b.running
}

func (b *recordingBackend) Create(context.Context, targets.Target, state.Workspace) error {
	b.actions = append(b.actions, "create")
	return nil
}

func (b *recordingBackend) Start(_ context.Context, _ targets.Target, _ state.Workspace, opts backend.StartOptions) error {
	b.running = true
	b.actions = append(b.actions, fmt.Sprintf("start headless=%t vnc=%t", opts.Headless, opts.VNC))
	return nil
}

func (b *recordingBackend) Stop(context.Context, state.Workspace) error {
	b.running = false
	b.actions = append(b.actions, "stop")
	return nil
}

func (b *recordingBackend) Destroy(context.Context, state.Workspace) error {
	b.actions = append(b.actions, "destroy")
	return nil
}

func (b *recordingBackend) IP(context.Context, state.Workspace, int) (string, error) {
	b.actions = append(b.actions, "ip")
	return "127.0.0.1", nil
}

func (b *recordingBackend) Exec(_ context.Context, _ targets.Target, _ state.Workspace, _ []string, opts backend.ExecOptions) (int, error) {
	b.actions = append(b.actions, "exec")
	if opts.Stdout != nil && b.execStdout != "" {
		if _, err := io.WriteString(opts.Stdout, b.execStdout); err != nil {
			return -1, err
		}
	}
	return b.execExit, nil
}
