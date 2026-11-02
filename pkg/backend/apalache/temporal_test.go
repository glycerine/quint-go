package apalache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestSourceToGoApalacheTemporalProperties(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") != "1" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for the bound-three temporal solver corpus")
	}
	t.Parallel()
	for _, test := range []struct {
		name, property string
		violation      bool
	}{
		{"eventually-violated", "eventually(x == 2)", true},
		{"eventually-satisfied", "eventually(x == 1)", false},
		{"leads-to-violated", "x == 0 leadsTo x == 2", true},
		{"leads-to-satisfied", "x == 0 leadsTo x == 1", false},
		{"always-violated", "always(x == 0)", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			compiled := compileSource(t, fmt.Sprintf("module M { var x:int action init=x'=0 action step=x'=1-x val safe=true temporal live=%s }", test.property))
			result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: 3, Invariants: []string{}, Temporal: []string{"live"}})
			if err != nil {
				t.Fatal(err)
			}
			if !test.violation {
				if result.Outcome != CheckSafe || len(result.Counterexamples) != 0 {
					t.Fatal("satisfied property", result)
				}
				return
			}
			if result.Outcome != CheckViolation || len(result.Counterexamples) == 0 {
				t.Fatal("temporal violation was lost", result)
			}
			for _, encoded := range result.Counterexamples {
				trace, err := quint.DecodeITFTrace(encoded)
				if err != nil || len(trace.States) < 3 || len(trace.States) > 4 || len(trace.Vars) != 1 || trace.Vars[0] != "x" {
					t.Fatal("invalid temporal trace", trace, err)
				}
				values := make([]int64, 0, len(trace.States))
				for index, state := range trace.States {
					if state.Kind != "app" || state.Opcode != "Rec" || len(state.Args) != 2 || state.Args[0].Str != "x" || state.Args[1].Kind != "int" {
						t.Fatal("generated checker state leaked into user trace", state)
					}
					value := state.Args[1].Int.Big().Int64()
					if value != int64(index%2) {
						t.Fatal("counterexample violates init or step", index, value)
					}
					values = append(values, value)
				}
				// The deterministic execution cycles between 0 and 1. The first
				// repeated value closes a valid lasso; every future value is again
				// 0 or 1, so eventual 2 and 0 leads-to 2 are both false. Always 0
				// is refuted by the observed second state, independently of which
				// valid loop the solver selected.
				if values[0] != values[2] || values[1] != 1 {
					t.Fatal("trace does not witness the requested temporal failure", values)
				}
				// The current service omits the loop index. If it supplies one,
				// validate its closing transition without requiring a particular
				// valid solver witness.
				if trace.Loop != nil {
					loop := *trace.Loop
					if loop < 0 || loop >= len(values) || values[loop] != 1-values[len(values)-1] {
						t.Fatal("invalid lasso closing transition", loop, values)
					}
				}
			}
		})
	}
}

func TestSourceToGoApalacheUnsupportedTemporalProperties(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, property, message string
	}{
		{"weak-fairness", "weakFair(step, x)", "fairness is not supported"},
		{"strong-fairness", "strongFair(step, x)", "fairness is not supported"},
		// The finite quantifier reaches the Go backend, which currently fails
		// assignment rewriting rather than issuing a dedicated unsupported-
		// temporal-quantifier message. Pin the observed error so this query can
		// never silently turn into a successful safety result.
		{"temporal-quantifier", "Set(0,1).forall(n => eventually(x == n))", "is not assigned a value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			compiled := compileSource(t, fmt.Sprintf("module M { var x:int action init=x'=0 action step=x'=1-x val safe=true temporal live=%s }", test.property))
			result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: 2, Invariants: []string{}, Temporal: []string{"live"}})
			var failure *CommandFailure
			if err == nil || !errors.As(err, &failure) || !strings.Contains(err.Error(), test.message) || result.Outcome != "" {
				t.Fatal("unsupported temporal query was accepted", result, err)
			}
		})
	}
}

func TestSourceToGoApalacheTemporalStutteringLasso(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") != "1" {
		t.Skip("set QUINT_CORPUS_TESTS=1 for the temporal stuttering-lasso solver check")
	}
	t.Parallel()
	compiled := compileSource(t, "module M { var x:bool action init=x'=false action step=x'=x val safe=true temporal live=eventually(x) }")
	result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: 1, Invariants: []string{}, Temporal: []string{"live"}})
	if err != nil || result.Outcome != CheckViolation || len(result.Counterexamples) == 0 {
		t.Fatal("stuttering temporal counterexample", result, err)
	}
	for _, encoded := range result.Counterexamples {
		trace, err := quint.DecodeITFTrace(encoded)
		if err != nil || len(trace.States) != 2 || len(trace.Vars) != 1 || trace.Vars[0] != "x" {
			t.Fatal("invalid stuttering trace", trace, err)
		}
		for _, state := range trace.States {
			if state.Kind != "app" || state.Opcode != "Rec" || len(state.Args) != 2 || state.Args[0].Str != "x" || state.Args[1].Kind != "bool" || state.Args[1].Bool {
				t.Fatal("trace violates init or the stuttering step", state)
			}
		}
		if trace.Loop != nil && (*trace.Loop < 0 || *trace.Loop >= len(trace.States)) {
			t.Fatal("invalid stuttering lasso loop index", *trace.Loop)
		}
		// This repeated false state is a closed lasso in which x never holds,
		// independently of the internal loop selector hidden by the service.
	}
}
