package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
)

type flagSpec struct {
	Target    bool
	Repo      bool
	JSON      bool
	VNC       bool
	Resources bool
}

func commandFlags(name string, spec flagSpec) (*flag.FlagSet, *options) {
	opts := &options{Headless: true}
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {
		fmt.Fprint(os.Stdout, commandUsage(name))
	}
	if spec.Target {
		fs.Var(targetFlag{opts: opts}, "target", "target name")
	}
	if spec.Repo {
		fs.StringVar(&opts.Repo, "repo", "", "repository root")
	}
	if spec.JSON {
		fs.BoolVar(&opts.JSON, "json", false, "emit JSON")
	}
	if spec.VNC {
		fs.BoolVar(&opts.VNC, "vnc", false, "start VM with VNC display")
	}
	if spec.Resources {
		fs.IntVar(&opts.CPU, "cpu", 0, "override VM CPU count for this workspace")
		fs.IntVar(&opts.MemoryMB, "memory-mb", 0, "override VM memory in MiB for this workspace")
		fs.IntVar(&opts.DiskGB, "disk-gb", 0, "override VM disk size in GiB for this workspace")
	}
	return fs, opts
}

func parseFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

type targetFlag struct {
	opts *options
}

func (f targetFlag) String() string {
	if f.opts == nil {
		return ""
	}
	return f.opts.Target
}

func (f targetFlag) Set(value string) error {
	f.opts.Target = value
	f.opts.TargetSet = true
	return nil
}
