package targets

import (
	"fmt"
	"strings"
)

type Target struct {
	Name         string   `json:"name"`
	Backend      string   `json:"-"`
	ImageName    string   `json:"-"`
	VMName       string   `json:"-"`
	OS           string   `json:"os"`
	Version      string   `json:"version"`
	Arch         string   `json:"arch"`
	Runnable     bool     `json:"runnable"`
	CPU          int      `json:"-"`
	MemoryMB     int      `json:"-"`
	DiskGB       int      `json:"-"`
	Display      string   `json:"-"`
	Username     string   `json:"-"`
	Password     string   `json:"-"`
	WorkPath     string   `json:"-"`
	Notes        string   `json:"notes,omitempty"`
	Capabilities []string `json:"capabilities"`
}

var builtins = map[string]Target{
	"macos10.15-x64": {
		Name:         "macos10.15-x64",
		Backend:      "reference",
		OS:           "macos",
		Version:      "10.15",
		Arch:         "x64",
		Runnable:     false,
		Notes:        "reference target; not runnable by the Tart Apple Silicon backend",
		Capabilities: []string{"reference-only"},
	},
	"macos14-x64": {
		Name:         "macos14-x64",
		Backend:      "reference",
		OS:           "macos",
		Version:      "14.x",
		Arch:         "x64",
		Runnable:     false,
		Notes:        "reference target for x64 macOS 14 systems",
		Capabilities: []string{"reference-only"},
	},
	"macos14-arm64": {
		Name:         "macos14-arm64",
		Backend:      "reference",
		OS:           "macos",
		Version:      "14.x",
		Arch:         "arm64",
		Runnable:     false,
		Notes:        "reference target; add a Trybox image before running locally",
		Capabilities: []string{"reference-only"},
	},
	"macos15-x64-rosetta": {
		Name:      "macos15-x64-rosetta",
		Backend:   "tart",
		ImageName: "trybox-macos15-arm64-image",
		VMName:    "trybox-macos15-x64-rosetta",
		OS:        "macos",
		Version:   "15.x",
		Arch:      "x64-rosetta",
		Runnable:  true,
		CPU:       4,
		MemoryMB:  8192,
		DiskGB:    100,
		Display:   "1920x1200",
		Username:  "admin",
		Password:  "admin",
		WorkPath:  "/Users/admin/trybox/work/firefox",
		Notes:     "runs on an arm64 macOS 15 VM; x64 behavior depends on Rosetta in the guest",
		Capabilities: []string{
			"clean-macos-vm",
			"manifest-rsync",
			"durable-run-logs",
			"rosetta",
		},
	},
	"macos15-arm64": {
		Name:      "macos15-arm64",
		Backend:   "tart",
		ImageName: "trybox-macos15-arm64-image",
		VMName:    "trybox-macos15-arm64",
		OS:        "macos",
		Version:   "15.x",
		Arch:      "arm64",
		Runnable:  true,
		CPU:       8,
		MemoryMB:  16384,
		DiskGB:    200,
		Display:   "1920x1200",
		Username:  "admin",
		Password:  "admin",
		WorkPath:  "/Users/admin/trybox/work/firefox",
		Capabilities: []string{
			"clean-macos-vm",
			"manifest-rsync",
			"durable-run-logs",
		},
	},
}

func Names() []string {
	return []string{
		"macos10.15-x64",
		"macos14-x64",
		"macos14-arm64",
		"macos15-x64-rosetta",
		"macos15-arm64",
	}
}

func List() []Target {
	out := make([]Target, 0, len(builtins))
	for _, name := range Names() {
		out = append(out, builtins[name])
	}
	return out
}

func Get(name string) (Target, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "macos15-arm64"
	}
	target, ok := builtins[name]
	if !ok {
		return Target{}, fmt.Errorf("unknown target %q; available targets: %s", name, strings.Join(Names(), ", "))
	}
	return target, nil
}
