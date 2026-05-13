package targets

import "testing"

func TestGetDefaultTarget(t *testing.T) {
	target, err := Get("")
	if err != nil {
		t.Fatal(err)
	}
	if target.Name != "macos15-arm64" || target.Backend != "tart" || !target.Runnable {
		t.Fatalf("Get(\"\") = %+v", target)
	}
}

func TestNamesAndListStayAligned(t *testing.T) {
	names := Names()
	list := List()
	if len(names) == 0 || len(names) != len(list) {
		t.Fatalf("Names() len = %d, List() len = %d", len(names), len(list))
	}
	seen := map[string]bool{}
	for _, target := range list {
		seen[target.Name] = true
	}
	for _, name := range names {
		if !seen[name] {
			t.Fatalf("Names() includes %q but List() does not", name)
		}
	}
}

func TestRosettaTargetDocumentsArchitecture(t *testing.T) {
	target, err := Get("macos15-x64-rosetta")
	if err != nil {
		t.Fatal(err)
	}
	if target.Arch != "x64-rosetta" || target.Notes == "" {
		t.Fatalf("Get(\"macos15-x64-rosetta\") = %+v", target)
	}
}
