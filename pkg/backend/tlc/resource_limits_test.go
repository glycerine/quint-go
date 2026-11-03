package tlc

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	native "github.com/glycerine/tlago/tlc"
)

// Native SetBound limits collection growth, rather than the number of states
// explored by the adapter's streaming breadth-first checker. These tests never
// approach an actual process memory/stack limit and restore the native setting.
func TestNativeTLCCollectionResourceLimit(t *testing.T) {
	native.Globals.Lock()
	previous := native.Globals.SetBound
	native.Globals.SetBound = 2
	native.Globals.Unlock()
	t.Cleanup(func() {
		native.Globals.Lock()
		native.Globals.SetBound = previous
		native.Globals.Unlock()
	})
	metadataRoot := t.TempDir()
	t.Setenv("TMPDIR", metadataRoot)
	t.Run("materialized set", func(t *testing.T) {
		source := `---- MODULE M ----
EXTENDS Integers, FiniteSets
VARIABLE x
q_init == x=Cardinality({n \in 1..11 : TRUE})
q_step == x'=x
====`
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		result, err := CheckTLA(ctx, "M.tla", source, Options{Workers: 1})
		if err == nil || result.Complete || result.Outcome != "" {
			t.Fatalf("resource exhaustion reported a definitive verdict: %+v, %v", result, err)
		}
		if !strings.Contains(result.Log+err.Error(), "Attempted to construct a set with too many elements (>2)") {
			t.Fatal("failure did not establish native collection exhaustion", result, err)
		}
		resourceMetadataEmpty(t, metadataRoot)
		native.Globals.Lock()
		bound := native.Globals.SetBound
		native.Globals.Unlock()
		if bound != 2 {
			t.Fatalf("checker changed caller's collection bound to %d", bound)
		}
	})

	// A successful control after exhaustion exercises ownership/worker cleanup
	// and shows that this bound is not a maximum explored-state count.
	result, err := CheckTLA(context.Background(), "M.tla", `---- MODULE M ----
VARIABLE x
q_init == x \in {0,1,2}
q_step == x'=x
====`, Options{Workers: 1})
	if err != nil || !result.Complete || result.Outcome != Safe || result.InitialStates != 3 || result.DistinctStates != 3 {
		t.Fatalf("collection limit was treated as a state budget: %+v, %v", result, err)
	}
	resourceMetadataEmpty(t, metadataRoot)

	// Successor states stream directly to the breadth-first checker, so even
	// temporal checking can explore more states than the collection-growth
	// bound. Its finite graph still has the genuine 0-stuttering counterexample.
	result, err = CheckTLA(context.Background(), "M.tla", `---- MODULE M ----
VARIABLE x
q_init == x=0
q_step == x' \in {0,1,2}
Live == <>(x=3)
====`, Options{Workers: 1, Temporal: []string{"Live"}})
	if err != nil || !result.Complete || result.Outcome != Violation || result.ErrorCode != native.ECTLCTemporalPropertyViolated || result.DistinctStates != 3 || len(result.Trace) == 0 || result.Loop == nil {
		t.Fatalf("streamed temporal model lost its actual violation: %+v, %v", result, err)
	}
	resourceMetadataEmpty(t, metadataRoot)
}

func resourceMetadataEmpty(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("TLC left temporary metadata after checking: %+v, %v", entries, err)
	}
	for _, entry := range entries {
		// Native FilenameToStream resolvers allocate separate tlc-* directories
		// without a public close API. Assert cleanup of the adapter-owned tree;
		// those process resolver allocations remain a native lifecycle gap.
		if strings.HasPrefix(entry.Name(), "quint-go-tlc-") {
			t.Fatalf("adapter left temporary metadata after checking: %s", entry.Name())
		}
	}
}
