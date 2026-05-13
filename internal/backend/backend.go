package backend

import (
	"context"
	"io"

	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

type StartOptions struct {
	RepoRoot string
	Headless bool
	VNC      bool
}

type ExecOptions struct {
	Workdir string
	Stdout  io.Writer
	Stderr  io.Writer
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
}

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}
