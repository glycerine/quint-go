package tlc

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

// tlaValueToITF converts the TLA-syntax values in a native trace to ITF JSON for
// the boolean and integer variables in the bounded fixtures.
func tlaValueToITF(t *testing.T, value string) any {
	t.Helper()
	switch value {
	case "TRUE":
		return true
	case "FALSE":
		return false
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatalf("unsupported witness value %q", value)
	}
	return map[string]string{"#bigint": fmt.Sprint(n)}
}

// TestBoundedTLCWitnessesValidate passes every TLC violation and deadlock trace
// from the bounded fixtures to the independent witness validator. The validator
// restates each spec by hand, so it does not trust TLC's own state evaluation.
func TestBoundedTLCWitnessesValidate(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not available for independent witness validation")
	}
	validator, err := filepath.Abs(filepath.Join("..", "..", "testdata", "reference", "bounds", "validate_witness.py"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var args []string
	for _, fixture := range boundedVerdicts {
		source, err := os.ReadFile(filepath.Join("testdata", "bounded", fixture.name+".tla"))
		if err != nil {
			t.Fatal(err)
		}
		for n := 1; n <= 4; n++ {
			outcome := fixture.outcomes[n-1]
			if outcome != Violation && outcome != Deadlock {
				continue
			}
			result, err := CheckTLA(context.Background(), fixture.name+".tla", string(source), Options{
				Init: "q_init", Next: "q_step", Invariants: []string{"q_inv"},
				CheckDeadlock: true, MaxSteps: n,
			})
			if err != nil {
				t.Fatalf("%s N=%d: %v", fixture.name, n, err)
			}
			if len(result.Trace) == 0 {
				t.Fatalf("%s N=%d: %s verdict has no counterexample trace", fixture.name, n, outcome)
			}
			witness := struct {
				States []map[string]any `json:"states"`
			}{}
			for _, state := range result.Trace {
				values := map[string]any{}
				for _, a := range state.Values {
					values[a.Name] = tlaValueToITF(t, a.Value)
				}
				witness.States = append(witness.States, values)
			}
			data, err := json.Marshal(witness)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, fmt.Sprintf("%s-%d.itf.json", fixture.name, n))
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			args = append(args, fixture.name+"="+path)
		}
	}
	if len(args) == 0 {
		t.Fatal("no violation or deadlock witnesses were produced")
	}
	out, err := exec.Command(python, append([]string{"-I", validator}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("witness validation failed: %v\n%s", err, out)
	}
	t.Logf("%s", out)
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
