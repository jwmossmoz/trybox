package targets

import (
	"fmt"
	"strings"
)

type Target struct {
	Name          string `json:"name"`
	Backend       string `json:"-"`
	ImageName     string `json:"-"`
	SourceImage   string `json:"-"`
	OS            string `json:"os"`
	Version       string `json:"version"`
	Arch          string `json:"arch"`
	Runnable      bool   `json:"runnable"`
	CPU           int    `json:"-"`
	MemoryMB      int    `json:"-"`
	DiskGB        int    `json:"-"`
	Display       string `json:"-"`
	Username      string `json:"-"`
	Password      string `json:"-"`
	GuestWorkPath string `json:"-"`
	Notes         string `json:"notes,omitempty"`
}

var macOSFamilies = []struct {
	major    string
	version  string
	codename string
}{
	{major: "12", version: "12.x", codename: "monterey"},
	{major: "13", version: "13.x", codename: "ventura"},
	{major: "14", version: "14.x", codename: "sonoma"},
	{major: "15", version: "15.x", codename: "sequoia"},
	{major: "26", version: "26.x", codename: "tahoe"},
}

var builtins = buildTargets()

func Names() []string {
	names := make([]string, 0, len(macOSFamilies)*2)
	for _, family := range macOSFamilies {
		names = append(names,
			"macos"+family.major+"-arm64",
			"macos"+family.major+"-x64-rosetta",
		)
	}
	return names
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

func buildTargets() map[string]Target {
	out := make(map[string]Target, len(macOSFamilies)*2)
	for _, family := range macOSFamilies {
		arm := macOSTarget(family.major, family.version, family.codename, "arm64")
		rosetta := macOSTarget(family.major, family.version, family.codename, "x64-rosetta")
		rosetta.Notes = "runs on an arm64 macOS VM; x64 behavior depends on Rosetta in the guest"
		out[arm.Name] = arm
		out[rosetta.Name] = rosetta
	}
	return out
}

func macOSTarget(major, version, codename, arch string) Target {
	name := "macos" + major + "-" + arch
	return Target{
		Name:          name,
		Backend:       "tart",
		ImageName:     "trybox-macos" + major + "-arm64-image",
		SourceImage:   "ghcr.io/cirruslabs/macos-" + codename + "-base:latest",
		OS:            "macos",
		Version:       version,
		Arch:          arch,
		Runnable:      true,
		CPU:           8,
		MemoryMB:      16384,
		DiskGB:        200,
		Display:       "1920x1200",
		Username:      "admin",
		Password:      "admin",
		GuestWorkPath: "/Users/admin/trybox",
	}
}
