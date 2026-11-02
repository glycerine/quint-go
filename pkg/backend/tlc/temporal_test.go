package tlc

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	native "github.com/glycerine/tlago/tlc"
)

// Even native LNCheck=off must not turn an unchecked temporal property into a
// proof. BOOLEAN avoids the comparatively expensive Integers module on the
// default suite; generated-source and richer lasso cases run in the corpus.
func TestNativeTLCTemporalAndInheritedPolicy(t *testing.T) {
	native.Globals.Lock()
	previous := native.Globals.LNCheck
	native.Globals.LNCheck = "off"
	native.Globals.Unlock()
	defer func() {
		native.Globals.Lock()
		native.Globals.LNCheck = previous
		native.Globals.Unlock()
	}()
	for _, tc := range []struct {
		property string
		outcome  Outcome
	}{{"<>x", Violation}, {"<>(~x)", Safe}} {
		source := strings.Replace(booleanTLA, "====", "Live == "+tc.property+"\nExtra == []TRUE\n====", 1)
		result, err := CheckTLA(context.Background(), "M.tla", source, Options{Workers: 2, Invariants: []string{"q_inv"}, Temporal: []string{"Live", "Extra"}})
		if err != nil || !result.Complete || result.Outcome != tc.outcome {
			t.Fatalf("%s: %+v err=%v", tc.property, result, err)
		}
		if tc.outcome == Violation {
			if result.Loop == nil || *result.Loop < 0 || *result.Loop >= len(result.Trace) {
				t.Fatalf("invalid temporal closing transition: %+v", result)
			}
			for _, state := range result.Trace {
				if len(state.Values) != 1 || state.Values[0].Name != "x" || state.Values[0].Value != "FALSE" {
					t.Fatalf("<>x counterexample must satisfy init and stutter at FALSE: %+v", result)
				}
			}
			// FALSE->FALSE is permitted by INIT/NEXT's implicit stuttering.
		} else if result.Loop != nil || len(result.Trace) != 0 {
			t.Fatalf("safe result contains a counterexample: %+v", result)
		}
		assertTemporalEngineRan(t, result)
		native.Globals.Lock()
		restored := native.Globals.LNCheck == "off"
		native.Globals.Unlock()
		if !restored {
			t.Fatal("caller liveness policy was not restored")
		}
	}
}

func TestNativeTLCTemporalRejectUnsupportedFeatures(t *testing.T) {
	for _, property := range []string{`\AA y : <>(y=x)`, `\EE y : <>(y=x)`, `\A y \in {x} : <>(x=y)`} {
		t.Run(property, func(t *testing.T) {
			source := strings.Replace(booleanTLA, "====", "Live == "+property+"\n====", 1)
			result, err := CheckTLA(context.Background(), "M.tla", source, Options{Workers: 1, Temporal: []string{"Live"}})
			if err == nil || result.Complete || result.Outcome != "" {
				t.Fatalf("unsupported temporal feature must not prove a property: %+v err=%v", result, err)
			}
			if !strings.Contains(result.Log+err.Error(), "temporal") && !strings.Contains(result.Log+err.Error(), "Temporal") && !strings.Contains(result.Log+err.Error(), "not a legal TLC value") {
				t.Fatalf("missing useful unsupported-feature diagnostic: %+v err=%v", result, err)
			}
		})
	}
}

func TestNativeTLCTemporalFiniteQuantifierCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC temporal quantifier corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range []struct {
		property string
		outcome  Outcome
	}{
		{`\A y \in BOOLEAN : <>(x=y)`, Violation},
		{`WF_x(q_step) => (\A y \in BOOLEAN : <>(x=y))`, Safe},
	} {
		source := strings.Replace(booleanTLA, "====", "Live == "+tc.property+"\n====", 1)
		result, err := CheckTLA(context.Background(), "M.tla", source, Options{Workers: 1, Temporal: []string{"Live"}})
		if err != nil || !result.Complete || result.Outcome != tc.outcome {
			t.Fatalf("property=%s result=%+v err=%v", tc.property, result, err)
		}
		assertTemporalEngineRan(t, result)
		if tc.outcome == Violation {
			if result.Loop == nil || *result.Loop < 0 || *result.Loop >= len(result.Trace) || len(result.Trace) == 0 || result.Trace[0].Values[0].Value != "FALSE" {
				t.Fatalf("invalid quantified temporal counterexample: %+v", result)
			}
			// A stuttering suffix at FALSE (or TRUE after the initial FALSE)
			// refutes eventual visitation of both values exactly when a value is
			// absent from the complete prefix and cycle.
			seenFalse, seenTrue := false, false
			for _, state := range result.Trace {
				if len(state.Values) != 1 || state.Values[0].Name != "x" {
					t.Fatal(result)
				}
				switch state.Values[0].Value {
				case "FALSE":
					seenFalse = true
				case "TRUE":
					seenTrue = true
				default:
					t.Fatal(result)
				}
			}
			if seenFalse && seenTrue {
				t.Fatalf("witness satisfies quantified eventuality: %+v", result)
			}
		}
	}
}

func TestGeneratedSourceTLCTemporalFiniteQuantifierCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("generated Quint temporal quantifier corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range []struct {
		property string
		outcome  Outcome
	}{
		{`Set(false,true).forall(y => eventually(x==y))`, Violation},
		{`weakFair(step,x) implies Set(false,true).forall(y => eventually(x==y))`, Safe},
	} {
		compiled := compileSource(t, `module M { var x:bool action init=x'=false action step=x'=not(x) val safe=true temporal live=`+tc.property+` }`, "live")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := Check(ctx, compiled, Options{Workers: 1, Temporal: []string{"q_temporalProps"}})
		cancel()
		if err != nil || !result.Complete || result.Outcome != tc.outcome {
			t.Fatalf("property=%s result=%+v err=%v", tc.property, result, err)
		}
		assertTemporalEngineRan(t, result)
		if tc.outcome == Violation {
			if result.Loop == nil || *result.Loop < 0 || *result.Loop >= len(result.Trace) || len(result.Trace) == 0 {
				t.Fatal(result)
			}
			for _, state := range result.Trace {
				if len(state.Values) != 1 || state.Values[0].Name != "x" || state.Values[0].Value != "FALSE" {
					t.Fatalf("quantified eventuality witness must omit TRUE: %+v", result)
				}
			}
		}
	}
}

type temporalCase struct {
	name, tla, quint, semantics string
	violation                   bool
}

// Every safe case has a direct finite-model argument: x stays in {0,1};
// eventually-zero is already true initially; reflexive leadsTo holds at its
// trigger; under WF/SF, the always-enabled toggle occurs infinitely often.
var temporalCases = []temporalCase{
	{"always satisfied", `[](x=0 \/ x=1)`, `always(x==0 or x==1)`, "always-domain", false},
	{"always violated", `[](x=0)`, `always(x==0)`, "always-zero", true},
	{"eventually satisfied", `<>(x=0)`, `eventually(x==0)`, "eventually-zero", false},
	{"eventually violated", `<>(x=2)`, `eventually(x==2)`, "eventually-two", true},
	{"stuttering prevents progress", `<>(x=1)`, `eventually(x==1)`, "eventually-one", true},
	{"leadsTo satisfied", `(x=0) ~> (x=0)`, `x==0 leadsTo x==0`, "leads-zero", false},
	{"leadsTo violated", `(x=0) ~> (x=2)`, `x==0 leadsTo x==2`, "leads-two", true},
	{"weakFair satisfied", `WF_x(q_step) => <>(x=1)`, `weakFair(step,x) implies eventually(x==1)`, "wf-eventually-one", false},
	{"weakFair violated", `WF_x(q_step)`, `weakFair(step,x)`, "wf", true},
	{"weakFair nonstuttering cycle", `WF_x(q_step) => <>(x=2)`, `weakFair(step,x) implies eventually(x==2)`, "wf-eventually-two", true},
	{"strongFair satisfied", `SF_x(q_step) => <>(x=1)`, `strongFair(step,x) implies eventually(x==1)`, "sf-eventually-one", false},
	{"strongFair violated", `SF_x(q_step)`, `strongFair(step,x)`, "sf", true},
	{"strongFair nonstuttering cycle", `SF_x(q_step) => <>(x=2)`, `strongFair(step,x) implies eventually(x==2)`, "sf-eventually-two", true},
}

const temporalToggleTLA = `---- MODULE M ----
EXTENDS Integers
VARIABLE x
q_init == x=0
q_step == x'=1-x
Live == PROPERTY_EXPRESSION
====`

func TestNativeTLCTemporalCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("native TLC temporal solver corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range temporalCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := CheckTLA(ctx, "M.tla", strings.Replace(temporalToggleTLA, "PROPERTY_EXPRESSION", tc.tla, 1), Options{Workers: 1, Temporal: []string{"Live"}})
			assertToggleTemporal(t, tc, result, err)
		})
	}
}

func TestGeneratedSourceTLCTemporalCorpus(t *testing.T) {
	if os.Getenv("QUINT_CORPUS_TESTS") == "" {
		t.Skip("generated Quint to native TLC temporal solver corpus; set QUINT_CORPUS_TESTS=1")
	}
	for _, tc := range temporalCases {
		t.Run(tc.name, func(t *testing.T) {
			compiled := compileSource(t, fmt.Sprintf("module M { var x:int action init=x'=0 action step=x'=1-x val safe=true temporal live=%s }", tc.quint), "live")
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result, err := Check(ctx, compiled, Options{Workers: 1, Temporal: []string{"q_temporalProps"}})
			assertToggleTemporal(t, tc, result, err)
		})
	}
}

func assertTemporalEngineRan(t *testing.T, result Result) {
	t.Helper()
	for _, message := range result.Messages {
		if message.Code == native.ECTLCCheckingTemporalProps {
			return
		}
	}
	t.Fatalf("native checker did not report executing liveness checking: %+v", result)
}

// Validate the actual witness rather than its exact choice or length. Model
// transitions are toggles or implicit stutters. Temporal evaluation below is
// independent of TLC's tableau and uses the finite prefix plus closing cycle.
func assertToggleTemporal(t *testing.T, tc temporalCase, result Result, err error) {
	t.Helper()
	want := Safe
	if tc.violation {
		want = Violation
	}
	if err != nil || !result.Complete || result.Outcome != want {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !strings.HasPrefix(tc.semantics, "always-") {
		assertTemporalEngineRan(t, result)
	}
	if want == Safe {
		if len(result.Trace) != 0 || result.Loop != nil || result.DistinctStates != 2 {
			t.Fatalf("safe result must exhaust the two-state graph: %+v", result)
		}
		return
	}
	values := make([]int, len(result.Trace))
	for i, state := range result.Trace {
		if state.Index != i || len(state.Values) != 1 || state.Values[0].Name != "x" {
			t.Fatalf("invalid state shape: %+v", result)
		}
		n, err := strconv.Atoi(state.Values[0].Value)
		if err != nil || n < 0 || n > 1 {
			t.Fatalf("state violates toggle domain: %+v", state)
		}
		values[i] = n
		if i > 0 && n != values[i-1] && n != 1-values[i-1] {
			t.Fatalf("invalid toggle/stutter transition: %+v", result.Trace)
		}
	}
	if len(values) == 0 || values[0] != 0 {
		t.Fatalf("counterexample does not satisfy init: %+v", result)
	}
	if tc.semantics == "always-zero" {
		for _, n := range values {
			if n != 0 {
				return
			}
		}
		t.Fatalf("counterexample does not violate always-zero: %+v", result)
	}
	if result.Loop == nil || *result.Loop < 0 || *result.Loop >= len(values) {
		t.Fatalf("temporal witness has no valid loop: %+v", result)
	}
	loop := *result.Loop
	last, closing := values[len(values)-1], values[loop]
	if closing != last && closing != 1-last {
		t.Fatalf("invalid closing transition: %+v", result)
	}
	contains := func(target int) bool {
		for _, n := range values {
			if n == target {
				return true
			}
		}
		return false
	}
	// q_step is continuously enabled at every state. Both WF and SF therefore
	// hold exactly when an actual toggle occurs somewhere on the repeated cycle.
	fair := closing != last
	for i := loop + 1; i < len(values); i++ {
		fair = fair || values[i] != values[i-1]
	}
	satisfied := false
	switch tc.semantics {
	case "eventually-one":
		satisfied = contains(1)
	case "eventually-two":
		satisfied = contains(2)
	case "leads-two":
		// Every trace begins with a trigger x=0 and never reaches x=2.
		satisfied = contains(2)
	case "wf", "sf":
		satisfied = fair
	case "wf-eventually-two", "sf-eventually-two":
		satisfied = !fair || contains(2)
	default:
		t.Fatalf("missing independent semantics for %q", tc.semantics)
	}
	if satisfied {
		t.Fatalf("counterexample actually satisfies %s: %+v", tc.semantics, result)
	}
}
