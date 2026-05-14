package cli

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestValidateSnapshotName(t *testing.T) {
	valid := []string{"flake-frozen", "a1", "debug-2026"}
	for _, name := range valid {
		if err := validateSnapshotName(name); err != nil {
			t.Fatalf("validateSnapshotName(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{"", "Flake", "-bad", "bad-", "bad--name", "bad_name"}
	for _, name := range invalid {
		if err := validateSnapshotName(name); err == nil {
			t.Fatalf("validateSnapshotName(%q) = nil, want error", name)
		}
	}
}

func TestSnapshotMetadataRoundTrip(t *testing.T) {
	store := testStore(t)
	empty, err := listSnapshots(store, "workspace_test")
	if err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("listSnapshots(empty) = %#v, want empty non-nil slice", empty)
	}
	meta := snapshotMeta{
		SchemaVersion:   1,
		Name:            "flake-frozen",
		WorkspaceID:     "workspace_test",
		Target:          "macos15-arm64",
		RepoRoot:        t.TempDir(),
		VMName:          "trybox-ws-test",
		SnapshotVMName:  "trybox-ws-test.snapshot.flake-frozen",
		SyncFingerprint: "abc123",
		CreatedAt:       time.Now().UTC(),
		NominalBytes:    10,
		DiskBytes:       5,
	}
	if err := saveSnapshot(store, meta); err != nil {
		t.Fatal(err)
	}
	got, err := loadSnapshot(store, meta.WorkspaceID, meta.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != meta.Name || got.SnapshotVMName != meta.SnapshotVMName || got.SyncFingerprint != meta.SyncFingerprint {
		t.Fatalf("loadSnapshot() = %+v, want %+v", got, meta)
	}
	list, err := listSnapshots(store, meta.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != meta.Name {
		t.Fatalf("listSnapshots() = %+v, want one %q", list, meta.Name)
	}
	if err := os.Remove(snapshotPath(store, meta.WorkspaceID, meta.Name)); err != nil {
		t.Fatal(err)
	}
	if _, err := loadSnapshot(store, meta.WorkspaceID, meta.Name); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("loadSnapshot(missing) error = %v, want os.ErrNotExist", err)
	}
}
