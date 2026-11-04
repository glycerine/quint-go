package apalache

import (
	"context"
	"fmt"
	"os"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

// Nondeterministic branching corpus for temporal checking. From every state,
// x either advances modulo 3 or resets to 0, so the staying path 0,0,0,... is
// a legal infinite execution that never reaches 2. Properties that a single
// path can refute are therefore violated, while properties that hold on every
// path are satisfied.
func TestSourceToGoApalacheBranchingTemporalCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") != "1" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for the branching temporal corpus")
	}
	t.Parallel()
	const next = "any { all { x < 2, x' = x + 1 }, all { x' = 0 } }"
	for _, test := range []struct {
		name, property string
		violation      bool
	}{
		{"eventually-refuted-by-stay-at-zero", "eventually(x == 2)", true},
		{"leads-to-refuted-by-stay-at-zero", "x == 0 leadsTo x == 2", true},
		{"always-in-range-satisfied", "always(x >= 0 and x <= 2)", false},
		{"leads-from-two-satisfied", "x == 2 leadsTo x == 0", false},
		{"always-implies-eventually-satisfied", "always(x == 0 implies eventually(x == 0))", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := fmt.Sprintf("module M { var x:int action init=x'=0 action step=%s val safe=true temporal live=%s }", next, test.property)
			compiled := compileSource(t, source)
			result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: 4, Invariants: []string{}, Temporal: []string{"live"}})
			if err != nil {
				t.Fatal(err)
			}
			if !test.violation {
				if result.Outcome != CheckSafe || len(result.Counterexamples) != 0 {
					t.Fatalf("satisfied property reported %+v", result)
				}
				return
			}
			if result.Outcome != CheckViolation || len(result.Counterexamples) == 0 {
				t.Fatalf("temporal violation was lost: %+v", result)
			}
			for _, encoded := range result.Counterexamples {
				trace, err := quint.DecodeITFTrace(encoded)
				if err != nil {
					t.Fatal(err)
				}
				values := branchingValues(t, trace)
				for index, value := range values {
					if index == 0 && value != 0 {
						t.Fatalf("trace does not start at init: %v", values)
					}
					if index > 0 && !legalBranchingStep(values[index-1], value) {
						t.Fatalf("illegal step %d -> %d in %v", values[index-1], value, values)
					}
				}
				if trace.Loop != nil {
					loop := *trace.Loop
					last := values[len(values)-1]
					if loop < 0 || loop >= len(values) || !legalBranchingStep(last, values[loop]) {
						t.Fatalf("invalid lasso closing transition loop=%d values=%v", loop, values)
					}
				}
			}
		})
	}
}

func branchingValues(t *testing.T, trace quint.ITFTrace) []int64 {
	t.Helper()
	if len(trace.Vars) != 1 || trace.Vars[0] != "x" {
		t.Fatalf("unexpected trace variables %v", trace.Vars)
	}
	values := make([]int64, 0, len(trace.States))
	for _, state := range trace.States {
		if state.Kind != "app" || state.Opcode != "Rec" || len(state.Args) != 2 || state.Args[0].Str != "x" || state.Args[1].Kind != "int" {
			t.Fatalf("generated checker state leaked into user trace: %+v", state)
		}
		values = append(values, state.Args[1].Int.Big().Int64())
	}
	return values
}

func legalBranchingStep(from, to int64) bool {
	if from < 2 {
		return to == from+1 || to == 0
	}
	return to == 0
}
