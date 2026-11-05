package apalache

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ap "github.com/glycerine/apalache-go"
)

// TestScalaTuningCensusOutcomesStable pins how the Go tuning parser classifies
// every key in Apalache v0.62.2's FineTuningParser table. Each row probes one
// key with a value of its Scala field type and records the outcome:
//
//	accept          - the Go parser and g3 apply the key
//	g3-unknown      - g3 has no such parameter or module, so the key is rejected
//	                  because it cannot be applied; this is intentional
//	non-z3-unknown  - a non-z3 key that the Go parser does not model
//	g3-rejected     - g3 rejects the probe for another reason
//
// A change in any outcome means either g3 or the Go parser changed. Review it
// before updating testdata/scala_tuning_census.tsv.
func TestScalaTuningCensusOutcomesStable(t *testing.T) {
	// The census includes z3.trace=true, which makes g3 write its default
	// "z3.log" trace file into the working directory. Run in a temporary
	// directory so validation never leaves that file in the package source tree.
	fixture, err := filepath.Abs("testdata/scala_tuning_census.tsv")
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	f, err := os.Open(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("malformed census row: %q", line)
		}
		key, value, want := fields[0], fields[2], fields[3]
		rows++
		_, err := ap.ParseCheckerTuning(map[string]string{key: value})
		var got string
		switch {
		case err == nil:
			got = "accept"
		case strings.Contains(err.Error(), "unknown module") || strings.Contains(err.Error(), "unknown parameter"):
			got = "g3-unknown"
		case strings.Contains(err.Error(), "unexpected tuning field"):
			got = "non-z3-unknown"
		default:
			got = "g3-rejected"
		}
		if got != want {
			t.Errorf("%s=%s: outcome %s, want %s (err=%v)", key, value, got, want, err)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != 484 {
		t.Fatalf("census has %d rows, want 484 keys from the Scala table", rows)
	}
}
