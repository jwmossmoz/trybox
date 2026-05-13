package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
)

func Run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		usage(os.Stdout)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 && args[0] == "help" {
			return printCommandHelp(args[1:])
		}
		usage(os.Stdout)
		return nil
	case "doctor":
		return doctor(ctx, args[1:])
	case "target":
		return target(ctx, args[1:])
	case "workspace":
		return workspaceCommand(ctx, args[1:])
	case "up":
		return up(ctx, args[1:])
	case "sync":
		return syncWorkspace(ctx, args[1:])
	case "status":
		return status(ctx, args[1:])
	case "view":
		return view(ctx, args[1:])
	case "stop":
		return stop(ctx, args[1:])
	case "destroy":
		return destroy(ctx, args[1:])
	case "run":
		return runCommand(ctx, args[1:])
	case "logs":
		return logs(ctx, args[1:])
	case "events":
		return events(ctx, args[1:])
	case "history":
		return history(ctx, args[1:])
	case "sync-plan":
		return syncPlan(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q; run trybox --help for usage", args[0])
	}
}

func Fatal(err error) {
	var exit exitError
	if errors.As(err, &exit) {
		os.Exit(exit.Code)
	}
	fmt.Fprintln(os.Stderr, "trybox:", err)
	os.Exit(1)
}

type exitError struct {
	Code int
}

func (e exitError) Error() string {
	return fmt.Sprintf("exit %d", e.Code)
}
