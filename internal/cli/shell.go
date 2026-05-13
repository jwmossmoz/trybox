package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func shell(ctx context.Context, args []string) error {
	fs, opts := commandFlags("shell", flagSpec{Target: true, Repo: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	command := fs.Args()
	if len(command) > 0 && command[0] == "--" {
		command = command[1:]
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	return withWorkspaceLock(ctx, store, workspace.ID, func() error {
		if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
			return err
		}
		keyPath, err := ensureSSHKey(ctx, target, workspace, b, store)
		if err != nil {
			return err
		}
		ip, err := b.IP(ctx, workspace, 120)
		if err != nil {
			return err
		}
		workspace.LastKnownIP = ip
		_ = store.SaveWorkspace(workspace)
		sshArgs := []string{
			"-i", keyPath,
			"-o", "IdentitiesOnly=yes",
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
			"-o", "LogLevel=ERROR",
			target.Username + "@" + ip,
		}
		if len(command) == 0 {
			command = []string{"cd " + shellQuote(remoteWorkPath(target)) + " && exec ${SHELL:-/bin/sh} -l"}
		}
		sshArgs = append(sshArgs, command...)
		cmd := exec.CommandContext(ctx, "ssh", sshArgs...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return exitError{Code: exitErr.ExitCode()}
			}
			return fmt.Errorf("ssh %s: %w", filepath.Base(keyPath), err)
		}
		return nil
	})
}
