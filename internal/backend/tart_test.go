package backend

import "testing"

func TestTartListSize(t *testing.T) {
	list := `Source Name                Disk Size Accessed       State
local  trybox-ws.snapshot.a 50   31   1 hour ago     stopped
`
	size, ok := tartListSize(list, "trybox-ws.snapshot.a")
	if !ok {
		t.Fatal("tartListSize() ok = false, want true")
	}
	const gib = int64(1024 * 1024 * 1024)
	if size.NominalBytes != 50*gib || size.DiskBytes != 31*gib {
		t.Fatalf("tartListSize() = %+v, want 50/31 GiB", size)
	}
}
