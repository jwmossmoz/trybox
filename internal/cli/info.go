package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/jwmossmoz/trybox/internal/targets"
)

func info(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprint(os.Stdout, commandUsage("info"))
	}
	jsonOut := fs.Bool("json", false, "emit JSON")
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: trybox info [--json]")
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	targetName := targetNameFor(nil, config)
	target, _ := targets.Get(targetName)
	var workspace any
	if config.DefaultWorkspaceID != "" {
		if value, err := store.LoadWorkspace(config.DefaultWorkspaceID); err == nil {
			view := viewWorkspace(value)
			workspace = view
		}
	}
	out := map[string]any{
		"state_root":           store.Root,
		"workspaces_dir":       store.WorkspacesDir,
		"runs_dir":             store.RunsDir,
		"logs_dir":             store.LogsDir,
		"keys_dir":             store.KeysDir,
		"config_path":          store.ConfigPath(),
		"default_target":       config.DefaultTarget,
		"default_repo_root":    config.DefaultRepoRoot,
		"default_workspace_id": config.DefaultWorkspaceID,
		"workspace":            workspace,
	}
	if target.Name != "" {
		out["target_image_name"] = target.ImageName
		out["target_source_image"] = target.SourceImage
		out["target_clone_command"] = targetCloneCommand(target)
	}
	if *jsonOut {
		_ = ctx
		return writeJSON(os.Stdout, out)
	}
	fmt.Printf("state root:          %s\n", store.Root)
	fmt.Printf("workspaces:          %s\n", store.WorkspacesDir)
	fmt.Printf("runs:                %s\n", store.RunsDir)
	fmt.Printf("logs:                %s\n", store.LogsDir)
	fmt.Printf("keys:                %s\n", store.KeysDir)
	fmt.Printf("config:              %s\n", store.ConfigPath())
	fmt.Printf("default target:      %s\n", config.DefaultTarget)
	fmt.Printf("default repo:        %s\n", config.DefaultRepoRoot)
	fmt.Printf("default workspace:   %s\n", config.DefaultWorkspaceID)
	if target.Name != "" {
		fmt.Printf("target image:        %s\n", target.ImageName)
		fmt.Printf("target source image: %s\n", target.SourceImage)
	}
	_ = ctx
	return nil
}
