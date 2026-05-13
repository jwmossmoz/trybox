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
	Create(ctx context.Context, target targets.Target, claim state.Claim) error
	Start(ctx context.Context, target targets.Target, claim state.Claim, opts StartOptions) error
	Stop(ctx context.Context, claim state.Claim) error
	Destroy(ctx context.Context, claim state.Claim) error
	IP(ctx context.Context, claim state.Claim, waitSeconds int) (string, error)
	Exec(ctx context.Context, target targets.Target, claim state.Claim, command []string, opts ExecOptions) (int, error)
}

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}
