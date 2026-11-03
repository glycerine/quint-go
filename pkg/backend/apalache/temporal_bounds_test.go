package apalache

import (
	"context"
	"os"
	"testing"
)

// Temporal checking searches for a lasso within the selected bound. These
// properties are false on the unique infinite execution, but the finite search
// has no closing loop at the selected bounds. A bounded safe result therefore
// must not be presented as an unbounded temporal proof.
func TestActualTemporalBoundedInterpretation(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") != "1" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for actual bounded temporal interpretation checks")
	}
	t.Parallel()
	for _, test := range []struct {
		name, next, property string
		bound                int
	}{
		{"zero-bound-has-no-loop", "1-x", "eventually(x==2)", 0},
		{"nonrepeating-prefix-has-no-loop", "x+1", "eventually(x<0)", 3},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			compiled := compileSource(t, "module M { var x:int action init=x'=0 action step=x'="+test.next+" val safe=true temporal live="+test.property+" }")
			result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: test.bound, Invariants: []string{}, Temporal: []string{"live"}})
			if err != nil || result.Outcome != CheckSafe || len(result.Counterexamples) != 0 {
				t.Fatalf("expected completed bounded search without a lasso: %+v, %v", result, err)
			}
		})
	}
}
