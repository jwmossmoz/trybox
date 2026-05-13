package backend

import (
	"context"
	"fmt"

	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

type Reference struct{}

func (r Reference) Name() string {
	return "reference"
}

func (r Reference) Doctor(ctx context.Context, target targets.Target) []Check {
	return []Check{
		{
			Name:   "target",
			OK:     false,
			Detail: fmt.Sprintf("%s is reference-only: %s", target.Name, target.Notes),
		},
	}
}

func (r Reference) Exists(ctx context.Context, vmName string) bool {
	return false
}

func (r Reference) IsRunning(ctx context.Context, vmName string) bool {
	return false
}

func (r Reference) Create(ctx context.Context, target targets.Target, claim state.Claim) error {
	return referenceErr(target)
}

func (r Reference) Start(ctx context.Context, target targets.Target, claim state.Claim, opts StartOptions) error {
	return referenceErr(target)
}

func (r Reference) Stop(ctx context.Context, claim state.Claim) error {
	return nil
}

func (r Reference) Destroy(ctx context.Context, claim state.Claim) error {
	return nil
}

func (r Reference) IP(ctx context.Context, claim state.Claim, waitSeconds int) (string, error) {
	return "", fmt.Errorf("reference target %q has no runnable VM", claim.Target)
}

func (r Reference) Exec(ctx context.Context, target targets.Target, claim state.Claim, command []string, opts ExecOptions) (int, error) {
	return -1, referenceErr(target)
}

func referenceErr(target targets.Target) error {
	return fmt.Errorf("target %q is reference-only and is not runnable by this backend: %s", target.Name, target.Notes)
}
