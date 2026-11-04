package tlc

import (
	"context"
	"strings"
	"testing"
	"time"

	goapalache "github.com/glycerine/quint-go/pkg/backend/apalache"
)

func TestGeneratedTLAAppliedLetLambdaSemantics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	compiled := compileSource(t, `module M {
  var x: int
  pure def apply(f: (int) => int): int = f(1)
  pure val result = apply(y => y + 1)
  action init = x' = 0
  action step = x' = x
  val safe = result == 2 and x == 0
  val wrong = result != 2 or x != 0
}`)
	tla, err := goapalache.New().TLA(ctx, compiled)
	if err != nil {
		t.Fatal("generate TLA:", err)
	}
	if !strings.Contains(tla, "LET __QUINT_LAMBDA_") || !strings.Contains(tla, "IN apply(__QUINT_LAMBDA_") {
		t.Fatalf("expected applied LET lambda in generated TLA:\n%s", tla)
	}
	for _, tc := range []struct {
		property string
		outcome  Outcome
		symbolic goapalache.CheckOutcome
	}{
		{property: "safe", outcome: Safe, symbolic: goapalache.CheckSafe},
		{property: "wrong", outcome: Violation, symbolic: goapalache.CheckViolation},
	} {
		generated, err := CheckTLA(ctx, "M.tla", tla, Options{Workers: 1, Init: "q_init", Next: "q_step", Invariants: []string{tc.property}})
		if err != nil || !generated.Complete || generated.Outcome != tc.outcome || tc.outcome == Safe && generated.InitialStates != 1 {
			t.Fatalf("generated TLA %s semantics: %+v, %v", tc.property, generated, err)
		}
		symbolic, err := goapalache.New().Check(ctx, compiled, nil, goapalache.CheckOptions{MaxSteps: 1, Invariants: []string{tc.property}})
		if err != nil || symbolic.Outcome != tc.symbolic {
			t.Fatalf("assignment IR %s semantics: %+v, %v", tc.property, symbolic, err)
		}
	}
}
