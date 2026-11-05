package tlc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestCheckLeavesNoTLAGOTempDirectories pins that a native TLC check leaves
// nothing behind under TMPDIR. tlago's SimpleFilenameToStream allocates a
// "tlc-*" directory per parse and never removes it, so checkTLA must route
// those allocations into its own removed metadata directory.
func TestCheckLeavesNoTLAGOTempDirectories(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	result, err := CheckTLA(context.Background(), "M.tla", booleanTLA, Options{Workers: 1, Invariants: []string{"q_inv"}})
	if err != nil || result.Outcome != Safe || !result.Complete {
		t.Fatal(result, err)
	}
	for _, pattern := range []string{"tlc-*", "quint-go-tlc-*"} {
		matches, err := filepath.Glob(filepath.Join(tmp, pattern))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 0 {
			t.Fatalf("temporary directories left behind for %s: %v", pattern, matches)
		}
	}
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("TMPDIR not empty after check: %v", entries)
	}
}
