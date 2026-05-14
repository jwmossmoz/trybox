package backend

import (
	"context"
	"io"

	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

type StartOptions struct {
	Headless bool
	VNC      bool
}

type ExecOptions struct {
	Workdir string
	Stdout  io.Writer
	Stderr  io.Writer
}

type SnapshotSize struct {
	NominalBytes int64
	DiskBytes    int64
}

type Backend interface {
	Name() string
	Doctor(ctx context.Context, target targets.Target) []Check
	Exists(ctx context.Context, vmName string) bool
	IsRunning(ctx context.Context, vmName string) bool
	Create(ctx context.Context, target targets.Target, workspace state.Workspace) error
	Start(ctx context.Context, target targets.Target, workspace state.Workspace, opts StartOptions) error
	Stop(ctx context.Context, workspace state.Workspace) error
	Destroy(ctx context.Context, workspace state.Workspace) error
	IP(ctx context.Context, workspace state.Workspace, waitSeconds int) (string, error)
	Exec(ctx context.Context, target targets.Target, workspace state.Workspace, command []string, opts ExecOptions) (int, error)
	SnapshotSave(ctx context.Context, target targets.Target, workspace state.Workspace, snapshotVMName string) error
	SnapshotRestore(ctx context.Context, target targets.Target, workspace state.Workspace, snapshotVMName string, opts StartOptions) error
	SnapshotDelete(ctx context.Context, snapshotVMName string) error
	SnapshotSize(ctx context.Context, snapshotVMName string) (SnapshotSize, error)
}

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}
