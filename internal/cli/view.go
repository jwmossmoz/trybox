package cli

import (
	"bytes"
	"context"
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

func view(ctx context.Context, args []string) error {
	fs, opts := commandFlags("view", flagSpec{Target: true, Repo: true, JSON: true, VNC: true})
	noOpen := fs.Bool("no-open", false, "print the VNC URL without opening Screen Sharing")
	fs.Bool("reuse-client", false, "accepted for compatibility; Trybox does not reset existing clients")
	restartDisplay := fs.Bool("restart-display", false, "restart a running VM to switch display mode")
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if opts.JSON {
		*noOpen = true
	}
	if *noOpen {
		opts.VNC = true
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	return withWorkspaceLock(ctx, store, workspace.ID, func() error {
		wasRunning := b.IsRunning(ctx, workspace.VMName)
		if wasRunning && !*restartDisplay {
			return fmt.Errorf("workspace VM %q is already running; rerun with --restart-display to restart it for %s", workspace.VMName, viewDisplayName(opts.VNC))
		}
		if err := b.Create(ctx, target, workspace); err != nil {
			return err
		}
		if !b.IsRunning(ctx, workspace.VMName) {
			if err := b.Start(ctx, target, workspace, backend.StartOptions{Headless: true}); err != nil {
				return err
			}
		}
		if _, err := b.IP(ctx, workspace, 120); err != nil {
			return err
		}
		if err := completeAutoLoginBootCycle(ctx, target, workspace, b); err != nil {
			return err
		}
		if b.IsRunning(ctx, workspace.VMName) {
			if err := b.Stop(ctx, workspace); err != nil {
				return err
			}
		}
		if err := b.Start(ctx, target, workspace, backend.StartOptions{VNC: opts.VNC}); err != nil {
			return err
		}
		ip, err := b.IP(ctx, workspace, 120)
		if err != nil {
			return err
		}
		workspace.LastKnownIP = ip
		if err := store.SaveWorkspace(workspace); err != nil {
			return err
		}
		displayURL := vncURL(ip, target.Username, "")
		openURL := displayURL
		if target.Password != "" {
			openURL = vncURL(ip, target.Username, target.Password)
		}
		out := map[string]any{
			"workspace":    viewWorkspace(workspace),
			"display":      viewDisplayName(opts.VNC),
			"client":       viewClientName(opts.VNC, *noOpen),
			"url":          displayURL,
			"fresh_client": false,
			"opened":       !*noOpen,
		}
		if opts.VNC && !*noOpen {
			if err := exec.CommandContext(ctx, "open", openURL).Start(); err != nil {
				return fmt.Errorf("open Screen Sharing failed: %w", err)
			}
		}
		if opts.JSON {
			return writeJSON(os.Stdout, out)
		}
		fmt.Printf("workspace: %s\ndisplay:   %s\nclient:    %s\n", workspace.ID, viewDisplayName(opts.VNC), viewClientName(opts.VNC, *noOpen))
		if opts.VNC {
			fmt.Printf("url:       %s\nusername:  %s\n", displayURL, target.Username)
			if *noOpen {
				fmt.Println("open:      skipped")
			} else {
				fmt.Println("open:      Screen Sharing launched")
			}
		} else {
			fmt.Println("open:      Tart native window launched")
		}
		return nil
	})
}

func viewDisplayName(vnc bool) string {
	if vnc {
		return "tart-vnc"
	}
	return "tart-native"
}

func viewClientName(vnc bool, noOpen bool) string {
	if !vnc {
		return "tart"
	}
	if noOpen {
		return "none"
	}
	return "screen-sharing"
}

func vncURL(host, username, password string) string {
	value := neturl.URL{Scheme: "vnc", Host: host}
	if username != "" && password != "" {
		value.User = neturl.UserPassword(username, password)
	} else if username != "" {
		value.User = neturl.User(username)
	}
	return value.String()
}

func completeAutoLoginBootCycle(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend) error {
	changed, err := ensureAutoLogin(ctx, target, workspace, b)
	if err != nil || !changed {
		return err
	}
	if b.IsRunning(ctx, workspace.VMName) {
		if err := b.Stop(ctx, workspace); err != nil {
			return err
		}
	}
	if err := b.Start(ctx, target, workspace, backend.StartOptions{Headless: true}); err != nil {
		return err
	}
	_, err = b.IP(ctx, workspace, 120)
	return err
}

func ensureAutoLogin(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend) (bool, error) {
	if target.Username == "" || target.Password == "" {
		return false, nil
	}
	expected := "Automatic login user: " + target.Username
	script := strings.Join([]string{
		"set -eu",
		"if sysadminctl -autologin status 2>&1 | grep -F " + shellQuote(expected) + " >/dev/null; then printf 'already\\n'; exit 0; fi",
		"printf '%s\\n' " + shellQuote(target.Password) + " | sudo -S sysadminctl -autologin set -userName " + shellQuote(target.Username) + " -password " + shellQuote(target.Password) + " -adminUser " + shellQuote(target.Username) + " -adminPassword " + shellQuote(target.Password) + " >/tmp/trybox-autologin.log 2>&1 || true",
		"sysadminctl -autologin status 2>&1 | grep -F " + shellQuote(expected) + " >/dev/null",
		"printf 'configured\\n'",
	}, "\n")
	var stdout bytes.Buffer
	exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", script}, backend.ExecOptions{
		Stdout: &stdout,
		Stderr: os.Stderr,
	})
	if err != nil {
		return false, err
	}
	if exitCode != 0 {
		return false, fmt.Errorf("macOS auto-login setup failed for %s; see /tmp/trybox-autologin.log in the guest", target.Username)
	}
	return strings.Contains(stdout.String(), "configured"), nil
}
