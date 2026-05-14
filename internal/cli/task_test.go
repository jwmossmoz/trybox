package cli

import "testing"

func TestRevisionFromInput(t *testing.T) {
	got := revisionFromInput("https://treeherder.example/jobs?repo=try&revision=abcdef1234567890")
	if got != "abcdef1234567890" {
		t.Fatalf("revisionFromInput(url) = %q", got)
	}
	got = revisionFromInput("push abcdef1234567890")
	if got != "abcdef1234567890" {
		t.Fatalf("revisionFromInput(text) = %q", got)
	}
}

func TestParseReplayFlags(t *testing.T) {
	opts, replay, rest, err := parseReplayFlags("task", []string{"--root-url", "https://tc.example", "--target=macos15-arm64", "task-id", "run", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if !replay.Run || replay.Shell || replay.RootURL != "https://tc.example" {
		t.Fatalf("replay flags = %+v", replay)
	}
	if !opts.JSON || !opts.TargetSet || opts.Target != "macos15-arm64" {
		t.Fatalf("options = %+v", opts)
	}
	if len(rest) != 1 || rest[0] != "task-id" {
		t.Fatalf("rest = %v", rest)
	}
}
