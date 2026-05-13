package cli

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/jwmossmoz/trybox/internal/backend"
	"github.com/jwmossmoz/trybox/internal/state"
	"github.com/jwmossmoz/trybox/internal/targets"
)

func fetch(ctx context.Context, args []string) error {
	fs, opts := commandFlags("fetch", flagSpec{Target: true, Repo: true, JSON: true})
	url := fs.String("url", "", "artifact URL to fetch from inside the guest")
	to := fs.String("to", "", "guest destination path")
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 0 || *url == "" || *to == "" {
		return fmt.Errorf("usage: trybox fetch --url URL --to guest-path [--target name] [--repo path] [--json]")
	}
	target, workspace, b, store, err := setup(opts)
	if err != nil {
		return err
	}
	result := fetchResult{
		WorkspaceID: workspace.ID,
		Target:      target.Name,
		URL:         *url,
		Destination: remoteDestination(target, *to),
	}
	if err := withWorkspaceLock(ctx, store, workspace.ID, func() error {
		if err := ensureVM(ctx, target, &workspace, b, store, opts); err != nil {
			return err
		}
		if _, err := ensureSSHKey(ctx, target, workspace, b, store); err != nil {
			return err
		}
		return fetchIntoGuest(ctx, target, workspace, b, *url, result.Destination)
	}); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, result)
	}
	fmt.Printf("fetched: %s -> %s\n", result.URL, result.Destination)
	return nil
}

func fetchIntoGuest(ctx context.Context, target targets.Target, workspace state.Workspace, b backend.Backend, url, destination string) error {
	dir := path.Dir(destination)
	cmd := strings.Join([]string{
		"mkdir -p " + shellQuote(dir),
		"curl -L --fail --output " + shellQuote(destination) + " " + shellQuote(url),
	}, " && ")
	exitCode, err := b.Exec(ctx, target, workspace, []string{"sh", "-lc", cmd}, backend.ExecOptions{
		Stdout: os.Stderr,
		Stderr: os.Stderr,
	})
	if err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("guest fetch failed with exit code %d", exitCode)
	}
	return nil
}

func remoteDestination(target targets.Target, destination string) string {
	return cleanRemoteDestination(remoteWorkPath(target), destination)
}

func cleanRemoteDestination(base, destination string) string {
	destination = strings.TrimSpace(destination)
	if strings.HasPrefix(destination, "/") {
		return path.Clean(destination)
	}
	return path.Join(base, destination)
}
