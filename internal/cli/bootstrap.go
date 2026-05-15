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
	replace := fs.Bool("replace", false, "replace existing target image")
	if handled, err := parseFlags(fs, args); handled || err != nil {
		return err
	}
	if len(fs.Args()) != 0 {
		return fmt.Errorf("usage: trybox bootstrap [--target name] [--replace] [--json]")
	}
	store, config, err := loadStoreConfig()
	if err != nil {
		return err
	}
	target, err := targets.Get(targetNameFor(opts, config))
	if err != nil {
		return err
	}
	if err := saveDefaultTargetIfSet(store, config, opts, target); err != nil {
		return err
	}
	if target.Backend != "tart" || target.SourceImage == "" || target.ImageName == "" {
		return fmt.Errorf("target %q does not support bootstrap yet", target.Name)
	}
	if _, err := exec.LookPath("tart"); err != nil {
		return fmt.Errorf("tart not found in PATH")
	}

	out, err := bootstrapTargetImage(ctx, target, backendFor(target), *replace, opts.JSON)
	if err != nil {
		return err
	}
	if opts.JSON {
		return writeJSON(os.Stdout, out)
	}
	return nil
}

func bootstrapTargetImage(ctx context.Context, target targets.Target, b interface {
	Exists(context.Context, string) bool
}, replace bool, quiet bool) (bootstrapView, error) {
	if _, err := exec.LookPath("tart"); err != nil {
		return bootstrapView{}, fmt.Errorf("tart not found in PATH")
	}
	alreadyPresent := b.Exists(ctx, target.ImageName)
	out := bootstrapView{
		Target:       target.Name,
		ImageName:    target.ImageName,
		SourceImage:  target.SourceImage,
		ImagePresent: alreadyPresent,
		Command:      targetBootstrapCommand(target),
	}
	if alreadyPresent && !replace {
		if !quiet {
			fmt.Printf("target:    %s\nimage:     %s\nsource:    %s\nstatus:    already present\n",
				target.Name, target.ImageName, target.SourceImage)
		}
		return out, nil
	}
	if alreadyPresent && replace {
		if !quiet {
			fmt.Printf("bootstrap: deleting existing image %s\n", target.ImageName)
		}
		if err := runBootstrapCommand(ctx, quiet, "tart", "delete", target.ImageName); err != nil {
			return out, err
		}
		out.Replaced = true
	}
	if !quiet {
		fmt.Printf("bootstrap: cloning %s -> %s\n", target.SourceImage, target.ImageName)
	} else {
		fmt.Fprintf(os.Stderr, "bootstrap: cloning %s -> %s\n", target.SourceImage, target.ImageName)
	}
	if err := runBootstrapCommand(ctx, quiet, "tart", "clone", target.SourceImage, target.ImageName); err != nil {
		return out, err
	}
	out.Cloned = true
	out.ImagePresent = true
	if !quiet {
		fmt.Printf("target:    %s\nimage:     %s\nsource:    %s\nstatus:    ready\n",
			target.Name, target.ImageName, target.SourceImage)
	}
	return out, nil
}

func runBootstrapCommand(ctx context.Context, quiet bool, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if quiet {
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s failed: %w", name, shellJoin(args), err)
	}
	return nil
}
