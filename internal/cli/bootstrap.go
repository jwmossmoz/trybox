package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/jwmossmoz/trybox/internal/targets"
)

func bootstrap(ctx context.Context, args []string) error {
	fs, opts := commandFlags("bootstrap", flagSpec{Target: true, JSON: true})
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: trybox bootstrap [--target name] [--json]")
	}
	_, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	target, err := targets.Get(targetNameFor(opts, config))
	if err != nil {
		return err
	}
	if target.Backend != "tart" {
		return fmt.Errorf("target %q does not support bootstrap yet", target.Name)
	}
	b := backendFor(target)
	alreadyPresent := b.Exists(ctx, target.ImageName)
	cloned := false
	if !alreadyPresent {
		cmd := exec.CommandContext(ctx, "tart", "clone", target.SourceImage, target.ImageName)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("bootstrap target image: %w", err)
		}
		cloned = true
	}
	out := map[string]any{
		"target":        target.Name,
		"image_name":    target.ImageName,
		"source_image":  target.SourceImage,
		"image_present": true,
		"cloned":        cloned,
		"clone_command": targetCloneCommand(target),
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	if alreadyPresent {
		fmt.Printf("target image already present: %s\n", target.ImageName)
		return nil
	}
	fmt.Printf("target image created: %s\n", target.ImageName)
	return nil
}
