package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	quint "github.com/glycerine/quint-go/pkg"
)

func TestActualNestedInstancesAndArbitraryIntegers(t *testing.T) {
	t.Parallel()
	const initial = "1234567890123456789012345678901234567890"
	const source = `module Counter {
  const start:int
  var x:int
  action init=x'=start
  action step=x'=x+1
  val safe=x<=start
}
module Middle {
  const base:int
  import Counter(start=base) as C
  action init=C::init
  action step=C::step
  val safe=C::safe
}
module M {
  import Middle(base=` + initial + `) as N
  action init=N::init
  action step=N::step
  val safe=N::safe
}`
	compiled := compileSource(t, source)
	before, err := json.Marshal(compiled)
	if err != nil {
		t.Fatal(err)
	}
	for bound := 0; bound <= 1; bound++ {
		result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: bound})
		if err != nil {
			t.Fatal(err)
		}
		if bound == 0 {
			if result.Outcome != CheckSafe || len(result.Counterexamples) != 0 {
				t.Fatalf("zero-bound initialization: %+v", result)
			}
			continue
		}
		if result.Outcome != CheckViolation || len(result.Counterexamples) == 0 {
			t.Fatalf("one-step violation lost: %+v", result)
		}
		for _, raw := range result.Counterexamples {
			trace, err := quint.DecodeITFTrace(raw)
			if err != nil || len(trace.Vars) != 1 || trace.Vars[0] != "M::N::Middle::C::x" || len(trace.States) != 2 {
				t.Fatalf("nested trace names/shape: %+v, %v", trace, err)
			}
			for i, state := range trace.States {
				want := initial
				if i == 1 {
					want = "1234567890123456789012345678901234567891"
				}
				if state.Opcode != "Rec" || len(state.Args) != 2 || state.Args[0].Str != trace.Vars[0] || state.Args[1].Kind != "int" || state.Args[1].Int.String() != want {
					t.Fatalf("trace violates initializer/transition/invariant: state %d = %+v", i, state)
				}
			}
		}
	}
	after, err := json.Marshal(compiled)
	if err != nil || string(before) != string(after) {
		t.Fatal("checking changed caller compilation", err)
	}
}

func TestActualUnsupportedInputCannotBecomeSafe(t *testing.T) {
	t.Parallel()
	compiled := compileSource(t, "module M { action init=true action step=true val safe=true }")
	for i := range compiled.MainModule.Declarations {
		if compiled.MainModule.Declarations[i].Name == "q::inv" {
			expr := *compiled.MainModule.Declarations[i].Expr
			expr.Kind, expr.Opcode = "app", "unsupportedPortProbe"
			expr.Args = []quint.Expr{}
			compiled.MainModule.Declarations[i].Expr = &expr
		}
	}
	result, err := New().Check(context.Background(), compiled, nil, CheckOptions{MaxSteps: 0})
	var failure *CommandFailure
	if err == nil || result.Outcome != "" || len(result.Counterexamples) != 0 {
		t.Fatalf("unsupported input produced verdict: %+v, %v", result, err)
	}
	if !strings.Contains(err.Error(), "unsupportedPortProbe") {
		t.Fatalf("unsupported opcode missing from diagnostic: %v (backend failure %v)", err, errors.As(err, &failure))
	}
}
