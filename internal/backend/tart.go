package backend

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/jwmossmoz/trybox/internal/sshx"
	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

type Tart struct {
	LogDir string
}

func (t Tart) Name() string {
	return "tart"
}

func (t Tart) Doctor(ctx context.Context, target targets.Target) []Check {
	checks := []Check{}
	if path, err := exec.LookPath("tart"); err == nil {
		checks = append(checks, Check{Name: "tart", OK: true, Detail: path})
	} else {
		checks = append(checks, Check{Name: "tart", OK: false, Detail: "tart not found in PATH"})
	}
	if runtimeOS() == "darwin" {
		checks = append(checks, Check{Name: "host-os", OK: true, Detail: "darwin"})
	} else {
		checks = append(checks, Check{Name: "host-os", OK: false, Detail: "Tart backend requires macOS host"})
	}
	if runtimeArch() == "arm64" {
		checks = append(checks, Check{Name: "host-arch", OK: true, Detail: "arm64"})
	} else {
		checks = append(checks, Check{Name: "host-arch", OK: false, Detail: "macOS Tart targets require Apple Silicon host"})
	}
	detail := "target image available"
	ok := t.Exists(ctx, target.ImageName)
	if !ok {
		detail = fmt.Sprintf("target image missing; run: tart clone %s %s", shellQuote(target.SourceImage), shellQuote(target.ImageName))
	}
	checks = append(checks, Check{Name: "target-image", OK: ok, Detail: detail})
	return checks
}

func (t Tart) Exists(ctx context.Context, vmName string) bool {
	out, err := tart(ctx, "list")
	if err != nil {
		return false
	}
	return listHasVM(out, vmName)
}

func (t Tart) IsRunning(ctx context.Context, vmName string) bool {
	out, err := tart(ctx, "list")
	if err != nil {
		return false
	}
	return vmState(out, vmName) == "running"
}

func (t Tart) Create(ctx context.Context, target targets.Target, workspace state.Workspace) error {
	if !t.Exists(ctx, target.ImageName) {
		return fmt.Errorf("target %q needs local target image %q; run: trybox bootstrap --target %s (or tart clone %s %s)",
			target.Name,
			target.ImageName,
			target.Name,
			shellQuote(target.SourceImage),
			shellQuote(target.ImageName),
		)
	}
	if t.Exists(ctx, workspace.VMName) {
		return nil
	}
	if _, err := tart(ctx, "clone", target.ImageName, workspace.VMName); err != nil {
		return err
	}
	_, err := tart(ctx,
		"set", workspace.VMName,
		"--cpu", strconv.Itoa(workspace.CPU),
		"--memory", strconv.Itoa(workspace.MemoryMB),
		"--display", target.Display,
		"--disk-size", strconv.Itoa(workspace.DiskGB),
		"--random-mac",
		"--random-serial",
	)
	return err
}

func (t Tart) Start(ctx context.Context, target targets.Target, workspace state.Workspace, opts StartOptions) error {
	if t.IsRunning(ctx, workspace.VMName) {
		return nil
	}

	if err := os.MkdirAll(t.LogDir, 0o700); err != nil {
		return err
	}
	logPath := filepath.Join(t.LogDir, workspace.VMName+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer logFile.Close()

	args := []string{"run", "--no-clipboard", "--no-audio", "--suspendable"}
	if opts.VNC {
		args = append(args, "--vnc")
	} else if opts.Headless {
		args = append(args, "--no-graphics")
	}
	args = append(args, workspace.VMName)

	cmd := exec.Command("tart", args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Process.Release(); err != nil {
		return err
	}
	return nil
}

func (t Tart) Stop(ctx context.Context, workspace state.Workspace) error {
	if !t.Exists(ctx, workspace.VMName) || !t.IsRunning(ctx, workspace.VMName) {
		return nil
	}
	_, err := tart(ctx, "stop", workspace.VMName)
	return err
}

func (t Tart) Destroy(ctx context.Context, workspace state.Workspace) error {
	if !t.Exists(ctx, workspace.VMName) {
		return nil
	}
	_ = t.Stop(ctx, workspace)
	_, err := tart(ctx, "delete", workspace.VMName)
	return err
}

func (t Tart) IP(ctx context.Context, workspace state.Workspace, waitSeconds int) (string, error) {
	args := []string{"ip", workspace.VMName}
	if waitSeconds > 0 {
		args = append(args, "--wait", strconv.Itoa(waitSeconds))
	}
	out, err := tart(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (t Tart) Exec(ctx context.Context, target targets.Target, workspace state.Workspace, command []string, opts ExecOptions) (int, error) {
	ip, err := t.IP(ctx, workspace, 120)
	if err != nil {
		return -1, err
	}
	remoteCommand := shellJoin(command)
	if opts.Workdir != "" {
		remoteCommand = "cd " + shellQuote(opts.Workdir) + " && " + remoteCommand
	}
	return sshx.Run(ctx, sshx.Config{
		Host:     ip,
		User:     target.Username,
		Password: target.Password,
	}, remoteCommand, opts.Stdout, opts.Stderr)
}

func (t Tart) SnapshotSave(ctx context.Context, target targets.Target, workspace state.Workspace, snapshotVMName string) error {
	if !t.Exists(ctx, workspace.VMName) {
		return fmt.Errorf("workspace VM %q does not exist; run trybox up first", workspace.VMName)
	}
	if t.Exists(ctx, snapshotVMName) {
		return fmt.Errorf("snapshot VM %q already exists", snapshotVMName)
	}
	wasRunning := t.IsRunning(ctx, workspace.VMName)
	if wasRunning {
		if _, err := tart(ctx, "suspend", workspace.VMName); err != nil {
			return err
		}
		defer func() {
			_ = t.Start(context.Background(), target, workspace, StartOptions{Headless: true})
		}()
	}
	if _, err := tart(ctx, "clone", workspace.VMName, snapshotVMName); err != nil {
		return err
	}
	return nil
}

func (t Tart) SnapshotRestore(ctx context.Context, target targets.Target, workspace state.Workspace, snapshotVMName string, opts StartOptions) error {
	if !t.Exists(ctx, snapshotVMName) {
		return fmt.Errorf("snapshot VM %q does not exist", snapshotVMName)
	}
	if err := t.Stop(ctx, workspace); err != nil {
		return err
	}
	if t.Exists(ctx, workspace.VMName) {
		if _, err := tart(ctx, "delete", workspace.VMName); err != nil {
			return err
		}
	}
	if _, err := tart(ctx, "clone", snapshotVMName, workspace.VMName); err != nil {
		return err
	}
	return t.Start(ctx, target, workspace, opts)
}

func (t Tart) SnapshotDelete(ctx context.Context, snapshotVMName string) error {
	if !t.Exists(ctx, snapshotVMName) {
		return nil
	}
	_, err := tart(ctx, "delete", snapshotVMName)
	return err
}

func (t Tart) SnapshotSize(ctx context.Context, snapshotVMName string) (SnapshotSize, error) {
	out, err := tart(ctx, "list")
	if err != nil {
		return SnapshotSize{}, err
	}
	size, ok := tartListSize(out, snapshotVMName)
	if !ok {
		return SnapshotSize{}, fmt.Errorf("snapshot VM %q does not exist", snapshotVMName)
	}
	return size, nil
}

func tart(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tart", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return stdout.String(), fmt.Errorf("tart %s failed: %w%s", strings.Join(args, " "), err, suffix(detail))
	}
	return stdout.String(), nil
}

func listHasVM(listOutput, vmName string) bool {
	return vmState(listOutput, vmName) != ""
}

func vmState(listOutput, vmName string) string {
	for _, line := range strings.Split(listOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 5 && fields[1] == vmName {
			return fields[len(fields)-1]
		}
	}
	return ""
}

func tartListSize(listOutput, vmName string) (SnapshotSize, bool) {
	for _, line := range strings.Split(listOutput, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[1] != vmName {
			continue
		}
		nominal, nominalErr := strconv.ParseInt(fields[2], 10, 64)
		disk, diskErr := strconv.ParseInt(fields[3], 10, 64)
		if nominalErr != nil || diskErr != nil {
			return SnapshotSize{}, false
		}
		const gib = int64(1024 * 1024 * 1024)
		return SnapshotSize{
			NominalBytes: nominal * gib,
			DiskBytes:    disk * gib,
		}, true
	}
	return SnapshotSize{}, false
}

func suffix(detail string) string {
	if detail == "" {
		return ""
	}
	return ": " + detail
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
