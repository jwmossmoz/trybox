package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

func viewTarget(target targets.Target, imagePresent bool) targetView {
	return targetView{
		Name:         target.Name,
		OS:           target.OS,
		Version:      target.Version,
		Arch:         target.Arch,
		Runnable:     target.Runnable,
		ImageName:    target.ImageName,
		SourceImage:  target.SourceImage,
		ImagePresent: imagePresent,
		CloneCommand: targetCloneCommand(target),
		Notes:        target.Notes,
	}
}

func targetCloneCommand(target targets.Target) string {
	if target.SourceImage == "" || target.ImageName == "" {
		return ""
	}
	return "tart clone " + shellQuote(target.SourceImage) + " " + shellQuote(target.ImageName)
}

func viewWorkspace(workspace state.Workspace) workspaceView {
	view := workspaceView{
		ID:              workspace.ID,
		Target:          workspace.Target,
		RepoRoot:        workspace.RepoRoot,
		VMName:          workspace.VMName,
		CPU:             workspace.CPU,
		MemoryMB:        workspace.MemoryMB,
		DiskGB:          workspace.DiskGB,
		CreatedAt:       workspace.CreatedAt,
		UpdatedAt:       workspace.UpdatedAt,
		LastRunLog:      workspace.LastRunLog,
		LastKnownIP:     workspace.LastKnownIP,
		SyncFingerprint: workspace.SyncFingerprint,
	}
	if !workspace.LastSyncAt.IsZero() {
		t := workspace.LastSyncAt
		view.LastSyncAt = &t
	}
	return view
}

func viewRun(run state.Run) runView {
	view := runView{
		ID:            run.ID,
		Target:        run.Target,
		RepoRoot:      run.RepoRoot,
		VMName:        run.VMName,
		Command:       run.Command,
		CommandString: shellJoin(run.Command),
		StartedAt:     run.StartedAt,
		EndedAt:       run.EndedAt,
		ExitCode:      run.ExitCode,
		OutputLog:     run.OutputLog,
		StdoutLog:     run.StdoutLog,
		StderrLog:     run.StderrLog,
		EventsLog:     run.EventsLog,
	}
	if !run.EndedAt.IsZero() {
		view.Duration = run.EndedAt.Sub(run.StartedAt).Round(time.Millisecond).String()
	}
	return view
}

func writeJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}

func printWarnings(warnings []string) {
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func shellJoin(args []string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func suffix(detail string) string {
	if detail == "" {
		return ""
	}
	return ": " + detail
}
