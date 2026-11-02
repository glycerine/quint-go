package tlc

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// Toggle is continuously enabled; Advance is enabled only while e is TRUE
// and x is FALSE. A fair alternating-e behavior can forever decline Advance
// under weak fairness, but strong fairness requires eventual x=TRUE.
func TestNativeTLCWeakVersusStrongFairnessCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC intermittent-enabling fairness corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, generated := range []bool{false, true} {
		for _, strong := range []bool{false, true} {
			t.Run(fmt.Sprintf("generated=%v/strong=%v", generated, strong), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				opts := Options{Workers: 1, Invariants: []string{}, Temporal: []string{"Live"}}
				var result Result
				var err error
				if generated {
					fairness := "weakFair"
					if strong {
						fairness = "strongFair"
					}
					source := fmt.Sprintf(`module M {
var x: bool
var e: bool
action init = all { x'=false, e'=false }
action toggle = all { e'=not(e), x'=x }
action advance = all { e, not(x), x'=true, e'=e }
action step = any { toggle, advance }
val safe = true
temporal live = (weakFair(toggle,e) and %s(advance,x)) implies eventually(x)
}`, fairness)
					opts.Temporal = []string{"q_temporalProps"}
					result, err = Check(ctx, compileSource(t, source, "live"), opts)
				} else {
					fairness := "WF"
					if strong {
						fairness = "SF"
					}
					source := strings.Replace(`---- MODULE M ----
VARIABLES x,e
q_init == /\ x=FALSE /\ e=FALSE
Toggle == /\ e'=~e /\ x'=x
Advance == /\ e /\ ~x /\ x'=TRUE /\ e'=e
q_step == Toggle \/ Advance
Live == (WF_e(Toggle) /\ FAIR_x(Advance)) => <>x
====`, "FAIR", fairness, 1)
					result, err = CheckTLA(ctx, "M.tla", source, opts)
				}
				want := Violation
				if strong {
					want = Safe
				}
				if err != nil || !result.Complete || result.Outcome != want {
					t.Fatalf("result=%+v err=%v", result, err)
				}
				assertTemporalEngineRan(t, result)
				if strong {
					if result.DistinctStates != 4 || len(result.Trace) != 0 || result.Loop != nil {
						t.Fatal(result)
					}
					return
				}
				if result.Loop == nil || *result.Loop < 0 || *result.Loop >= len(result.Trace) {
					t.Fatal(result)
				}
				enabled := make([]bool, len(result.Trace))
				for i, state := range result.Trace {
					if state.Index != i || len(state.Values) != 2 || state.Values[0].Name != "x" || state.Values[1].Name != "e" {
						t.Fatal(result)
					}
					// <>x must be false throughout the prefix and cycle. Given
					// x=FALSE, every e transition is a Toggle or a stutter.
					if state.Values[0].Value != "FALSE" {
						t.Fatalf("trace satisfies <>x: %+v", result)
					}
					switch state.Values[1].Value {
					case "FALSE":
						enabled[i] = false
					case "TRUE":
						enabled[i] = true
					default:
						t.Fatal(result)
					}
				}
				if enabled[0] {
					t.Fatalf("invalid initial state: %+v", result)
				}
				// The cycle must repeatedly enable AND disable Advance. Thus
				// Toggle is weakly fair and Advance weakly fair (not continuously
				// enabled), while Advance is not strongly fair.
				seenEnabled, seenDisabled := false, false
				for _, e := range enabled[*result.Loop:] {
					seenEnabled = seenEnabled || e
					seenDisabled = seenDisabled || !e
				}
				if !seenEnabled || !seenDisabled {
					t.Fatalf("counterexample fails its fairness premise: %+v", result)
				}
			})
		}
	}
}
