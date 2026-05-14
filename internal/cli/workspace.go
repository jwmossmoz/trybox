package cli

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/jwmossmoz/trybox/internal/targets"
	workspacepkg "github.com/jwmossmoz/trybox/internal/workspace"
)

func workspaceCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stdout, commandUsage("workspace"))
		_ = ctx
		return nil
	}
	if isHelp(args[0]) {
		fmt.Fprint(os.Stdout, commandUsage("workspace"))
		_ = ctx
		return nil
	}
	if len(args) > 1 && isHelp(args[1]) {
		return printCommandHelp([]string{"workspace", args[0]})
	}
	switch args[0] {
	case "use":
		return workspaceUse(ctx, args[1:])
	case "show":
		return workspaceShow(ctx, args[1:])
	case "list":
		return workspaceList(ctx, args[1:])
	case "unset":
		return workspaceUnset(ctx, args[1:])
	default:
		return fmt.Errorf("unknown workspace subcommand %q; run trybox help workspace for usage", args[0])
	}
}

func workspaceUse(ctx context.Context, args []string) error {
	fs, opts := commandFlags("workspace use", flagSpec{Target: true, JSON: true, Resources: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) > 1 {
		return fmt.Errorf("usage: trybox workspace use [repo]")
	}
	repoInput := opts.Repo
	if len(rest) == 1 {
		repoInput = rest[0]
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	repo, err := resolveRepoForUse(repoInput)
	if err != nil {
		return err
	}
	if err := workspacepkg.ValidateRepoRoot(repo); err != nil {
		return err
	}
	target, err := targets.Get(targetNameFor(opts, config))
	if err != nil {
		return err
	}
	workspace, err := loadOrCreateWorkspace(store, target, repo)
	if err != nil {
		return err
	}
	if resourceOverridesRequested(opts) && backendFor(target).Exists(ctx, workspace.VMName) {
		return fmt.Errorf("resource changes require destroying existing workspace VM %q first; run: trybox destroy %s", workspace.VMName, workspace.ID)
	}
	if err := applyResourceOverrides(&workspace, target, opts); err != nil {
		return err
	}
	if err := store.SaveWorkspace(workspace); err != nil {
		return err
	}
	config.DefaultTarget = target.Name
	config.DefaultRepoRoot = repo
	config.DefaultWorkspaceID = workspace.ID
	if err := store.SaveConfig(config); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, map[string]any{
			"default_target":       config.DefaultTarget,
			"default_repo_root":    config.DefaultRepoRoot,
			"default_workspace_id": config.DefaultWorkspaceID,
			"workspace":            viewWorkspace(workspace),
		})
	}
	fmt.Printf("default workspace: %s\ntarget:            %s\nrepo:              %s\nvm:                %s\n", workspace.ID, target.Name, repo, workspace.VMName)
	_ = ctx
	return nil
}

func workspaceShow(ctx context.Context, args []string) error {
	fs, opts := commandFlags("workspace show", flagSpec{JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	var workspace *workspaceView
	if config.DefaultWorkspaceID != "" {
		if value, err := store.LoadWorkspace(config.DefaultWorkspaceID); err == nil {
			view := viewWorkspace(value)
			workspace = &view
		}
	}
	out := map[string]any{
		"default_target":       config.DefaultTarget,
		"default_repo_root":    config.DefaultRepoRoot,
		"default_workspace_id": config.DefaultWorkspaceID,
		"workspace":            workspace,
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	if config.DefaultWorkspaceID == "" {
		fmt.Println("default workspace: unset")
		return nil
	}
	fmt.Printf("default workspace: %s\ntarget:            %s\nrepo:              %s\n", config.DefaultWorkspaceID, config.DefaultTarget, config.DefaultRepoRoot)
	if workspace != nil {
		fmt.Printf("vm:                %s\n", workspace.VMName)
	}
	_ = ctx
	return nil
}

func workspaceList(ctx context.Context, args []string) error {
	fs, opts := commandFlags("workspace list", flagSpec{JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: trybox workspace list [--json]")
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	workspaces, err := store.ListWorkspaces()
	if err != nil {
		return err
	}
	sort.Slice(workspaces, func(i, j int) bool {
		return workspaces[i].ID < workspaces[j].ID
	})
	if opts.JSON {
		entries := make([]map[string]any, 0, len(workspaces))
		for _, workspace := range workspaces {
			entries = append(entries, map[string]any{
				"workspace":  viewWorkspace(workspace),
				"is_default": workspace.ID == config.DefaultWorkspaceID,
			})
		}
		_ = ctx
		return writeJSON(os.Stdout, map[string]any{
			"default_workspace_id": config.DefaultWorkspaceID,
			"workspaces":           entries,
		})
	}
	if len(workspaces) == 0 {
		fmt.Println("no workspaces")
		_ = ctx
		return nil
	}
	for _, workspace := range workspaces {
		marker := "  "
		if workspace.ID == config.DefaultWorkspaceID {
			marker = "* "
		}
		fmt.Printf("%s%s\n    target: %s\n    repo:   %s\n    vm:     %s\n", marker, workspace.ID, workspace.Target, workspace.RepoRoot, workspace.VMName)
	}
	_ = ctx
	return nil
}

func workspaceUnset(ctx context.Context, args []string) error {
	fs, opts := commandFlags("workspace unset", flagSpec{JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	store, _, err := loadStoreConfig()
	if err != nil {
		return err
	}
	config, err := store.LoadConfig()
	if err != nil {
		return err
	}
	config.DefaultWorkspaceID = ""
	if err := store.SaveConfig(config); err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, map[string]any{"default_workspace_id": ""})
	}
	fmt.Println("default workspace: unset")
	_ = ctx
	return nil
}
