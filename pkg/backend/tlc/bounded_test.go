package tlc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// boundedVerdicts holds the TLC verdict for transition bounds N=1..4. These
// match the Apalache --length=N verdicts recorded in
// pkg/testdata/reference/bounds, which the reference checker produced on these
// same fixtures. Bound N is checked as DFID depth N+1, which counts the initial
// state. N=0 has no equivalent TLC run, because DFID depth 1 is rejected.
var boundedVerdicts = []struct {
	name     string
	outcomes [4]Outcome // index N-1
}{
	{"counter3", [4]Outcome{Safe, Safe, Violation, Violation}},
	{"guard", [4]Outcome{Safe, Safe, Safe, Violation}},
	{"jump", [4]Outcome{Safe, Safe, Violation, Violation}},
	{"safe", [4]Outcome{Safe, Safe, Safe, Safe}},
	{"sum", [4]Outcome{Safe, Safe, Safe, Safe}},
	{"tlcCounter", [4]Outcome{Safe, Safe, Safe, Deadlock}},
	{"toggle", [4]Outcome{Violation, Violation, Violation, Violation}},
	{"twovar", [4]Outcome{Safe, Safe, Violation, Violation}},
}

// TestBoundedVerdictsMatchReferenceChecker checks TLC's bounded verdicts. The
// deadlock in tlcCounter at N=4 needs CheckDeadlock, as in the reference run.
func TestBoundedVerdictsMatchReferenceChecker(t *testing.T) {
	for _, fixture := range boundedVerdicts {
		t.Run(fixture.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata", "bounded", fixture.name+".tla"))
			if err != nil {
				t.Fatal(err)
			}
			for n := 1; n <= 4; n++ {
				result, err := CheckTLA(context.Background(), fixture.name+".tla", string(source), Options{
					Init: "q_init", Next: "q_step", Invariants: []string{"q_inv"},
					CheckDeadlock: true, MaxSteps: n,
				})
				if err != nil {
					t.Fatalf("N=%d: %v", n, err)
				}
				if !result.Complete {
					t.Fatalf("N=%d: incomplete result", n)
				}
				if want := fixture.outcomes[n-1]; result.Outcome != want {
					t.Fatalf("N=%d: outcome %q, want %q", n, result.Outcome, want)
				}
			}
		})
	}
}

func TestBoundedOptionsValidation(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options Options
	}{
		{"negative steps", Options{MaxSteps: -1}},
		{"multiple workers", Options{MaxSteps: 2, Workers: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := CheckTLA(context.Background(), "M.tla", booleanTLA, tc.options); err == nil {
				t.Fatal("invalid bounded options were accepted")
			}
		})
	}
}
