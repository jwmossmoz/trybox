package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
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
		fs.StringVar(&opts.Profile, "profile", "", "named VM resource profile: test or build")
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

func parseInterspersedFlags(fs *flag.FlagSet, args []string) (bool, error) {
	normalized, err := normalizeInterspersedFlags(fs, args)
	if err != nil {
		return false, err
	}
	return parseFlags(fs, normalized)
}

func normalizeInterspersedFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	flags := []string{}
	positionals := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		name, hasValue := flagName(arg)
		def := fs.Lookup(name)
		if def == nil {
			flags = append(flags, arg)
			continue
		}
		flags = append(flags, arg)
		if hasValue || isBoolFlag(def) {
			continue
		}
		i++
		if i >= len(args) {
			continue
		}
		flags = append(flags, args[i])
	}
	return append(flags, positionals...), nil
}

func flagName(arg string) (string, bool) {
	name := strings.TrimLeft(arg, "-")
	if idx := strings.IndexByte(name, '='); idx >= 0 {
		return name[:idx], true
	}
	return name, false
}

type boolFlag interface {
	IsBoolFlag() bool
}

func isBoolFlag(def *flag.Flag) bool {
	value, ok := def.Value.(boolFlag)
	return ok && value.IsBoolFlag()
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
