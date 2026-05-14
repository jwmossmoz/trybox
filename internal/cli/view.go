package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

var tartVNCURLPattern = regexp.MustCompile(`vnc://[^\s]+`)

func view(ctx context.Context, args []string) error {
	fs, opts := commandFlags("view", flagSpec{Target: true, Repo: true, JSON: true, VNC: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	return withWorkspaceLock(ctx, store, workspace.ID, func() error {
		if !opts.JSON {
			fmt.Fprintf(os.Stderr, "vm:        ensuring %s\n", workspace.VMName)
		}
		if err := b.Create(ctx, target, workspace); err != nil {
			return err
		}
		if !b.IsRunning(ctx, workspace.VMName) {
			if !opts.JSON {
				fmt.Fprintln(os.Stderr, "vm:        starting headless for setup")
			}
			if err := b.Start(ctx, target, workspace, backend.StartOptions{Headless: true}); err != nil {
				return err
			}
		}
		if _, err := b.IP(ctx, workspace, 120); err != nil {
			return err
		}
		if !opts.JSON {
			fmt.Fprintln(os.Stderr, "login:     ensuring auto-login")
		}
		if err := ensureAutoLogin(ctx, target, workspace, b); err != nil {
			return err
		}
		if b.IsRunning(ctx, workspace.VMName) {
			if !opts.JSON {
				fmt.Fprintln(os.Stderr, "vm:        restarting for display mode")
			}
			if err := b.Stop(ctx, workspace); err != nil {
				return err
			}
		}
		logPath := filepath.Join(store.LogsDir, workspace.VMName+".log")
		logOffset := int64(0)
		if opts.VNC {
			logOffset = fileSize(logPath)
		}
		if err := b.Start(ctx, target, workspace, backend.StartOptions{VNC: opts.VNC}); err != nil {
			return err
		}
		if !opts.JSON {
			fmt.Fprintf(os.Stderr, "display:   starting %s\n", viewDisplayName(opts.VNC))
		}
		ip, err := b.IP(ctx, workspace, 120)
		if err != nil {
			return err
		}
		workspace.LastKnownIP = ip
		if err := store.SaveWorkspace(workspace); err != nil {
			return err
		}
		displayURL := ""
		if opts.VNC {
			displayURL, err = waitForTartVNCURL(ctx, logPath, logOffset, 15*time.Second)
			if err != nil {
				return err
			}
			closeScreenSharing(ctx)
		}
		out := map[string]any{
			"vm":           viewWorkspace(workspace),
			"display":      viewDisplayName(opts.VNC),
			"client":       viewClientName(opts.VNC),
			"url":          displayURL,
			"fresh_client": false,
			"opened":       !opts.VNC,
		}
		if opts.JSON {
			return writeJSON(os.Stdout, out)
		}
		fmt.Printf("vm:        %s\ndisplay:   %s\nclient:    %s\n", workspace.VMName, viewDisplayName(opts.VNC), viewClientName(opts.VNC))
		if opts.VNC {
			fmt.Printf("url:       %s\n", displayURL)
			fmt.Println("open:      skipped")
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

func viewClientName(vnc bool) string {
	if !vnc {
		return "tart"
	}
	return "none"
}

func waitForTartVNCURL(ctx context.Context, logPath string, offset int64, timeout time.Duration) (string, error) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		data, err := os.ReadFile(logPath)
		if err == nil {
			start := offset
			if start < 0 || start > int64(len(data)) {
				start = 0
			}
			if url := latestTartVNCURL(string(data[start:])); url != "" {
				return url, nil
			}
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-deadline.C:
			return "", fmt.Errorf("timed out waiting for Tart VNC endpoint in %s", logPath)
		case <-ticker.C:
		}
	}
}

func latestTartVNCURL(logText string) string {
	matches := tartVNCURLPattern.FindAllString(logText, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimRight(matches[len(matches)-1], ".")
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func closeScreenSharing(ctx context.Context) {
	_ = exec.CommandContext(ctx, "osascript", "-e", `tell application "Screen Sharing" to quit`).Run()
}

func ensureAutoLogin(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend) error {
	if target.Username == "" || target.Password == "" {
		return nil
	}
	expected := "Automatic login user: " + target.Username
	script := strings.Join([]string{
		"set -eu",
		"if sysadminctl -autologin status 2>&1 | grep -F " + shellQuote(expected) + " >/dev/null; then exit 0; fi",
		"printf '%s\\n' " + shellQuote(target.Password) + " | sudo -S sysadminctl -autologin set -userName " + shellQuote(target.Username) + " -password " + shellQuote(target.Password) + " -adminUser " + shellQuote(target.Username) + " -adminPassword " + shellQuote(target.Password) + " >/tmp/trybox-autologin.log 2>&1 || true",
		"sysadminctl -autologin status 2>&1 | grep -F " + shellQuote(expected) + " >/dev/null",
	}, "\n")
	exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", script}, backend.ExecOptions{
		Stdout: io.Discard,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("macOS auto-login setup failed for %s; see /tmp/trybox-autologin.log in the guest", target.Username)
	}
	return nil
}
