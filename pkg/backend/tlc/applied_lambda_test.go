package tlc

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	goapalache "github.com/glycerine/quint-go/pkg/backend/apalache"
)

func TestGeneratedTLAAppliedLambdaAndSignedArithmeticSemantics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	compiled := compileSource(t, `module M {
  var x: int
  pure def apply(f: (int) => int): int = f(1)
  pure def subtract(value: int, acc: int): int = value - acc
  pure val result = apply(y => y + 1)
  pure val foldedRight = List(1, 2, 3).foldr(0, (value, acc) => value - acc)
  pure val foldedRightNamed = List(1, 2, 3).foldr(0, subtract)
  pure val boundedLists = allListsUpTo(Set(1, 2), 2)
  pure val zeroBoundLists = allListsUpTo(Set(1), 0)
  pure val negativeBoundLists = allListsUpTo(Set(1), -1)
  pure val signedDivision = (-7 / 3 == -2) and (7 / -3 == -2) and (-7 / -3 == 2) and (7 / 3 == 2)
  pure val signedRemainder = (-7 % 3 == -1) and (7 % -3 == 1) and (-7 % -3 == -1) and (7 % 3 == 1)
  action init = x' = 0
  action step = x' = x
  val lambdaSafe = result == 2 and foldedRight == 2 and foldedRightNamed == 2 and x == 0
  val lambdaWrong = result != 2 or foldedRight != 2 or foldedRightNamed != 2 or x != 0
  val listsSafe = boundedLists.size() == 7 and boundedLists.contains(List(2, 1)) and boundedLists.contains(List()) and zeroBoundLists.size() == 1 and negativeBoundLists.size() == 1 and x == 0
  val listsWrong = boundedLists.size() != 7 or not(boundedLists.contains(List(2, 1))) or x != 0
  val divisionSafe = signedDivision and x == 0
  val remainderSafe = signedRemainder and x == 0
  val safe = lambdaSafe and listsSafe and divisionSafe and remainderSafe
  val wrong = lambdaWrong or listsWrong or not(signedDivision) or not(signedRemainder) or x != 0
}`)
	before, err := json.Marshal(compiled.MainModule)
	if err != nil {
		t.Fatal(err)
	}
	tla, err := goapalache.New().TLA(ctx, compiled)
	if err != nil {
		t.Fatal("generate TLA:", err)
	}
	if !strings.Contains(tla, "LET __QUINT_LAMBDA_") || !strings.Contains(tla, "IN apply(__QUINT_LAMBDA_") || !strings.Contains(tla, `\div`) || strings.Contains(tla, "%") || strings.Contains(tla, "foldr(") || strings.Contains(tla, "allListsUpTo(") {
		t.Fatalf("expected applied LET lambda, lowered foldr/allListsUpTo, truncating division, and derived remainder in generated TLA:\n%s", tla)
	}
	after, err := json.Marshal(compiled.MainModule)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("TLA adaptation mutated the caller's compilation", err)
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
			t.Fatalf("generated TLA %s semantics: %+v, %v\n%s", tc.property, generated, err, tla)
		}
		symbolic, err := goapalache.New().Check(ctx, compiled, nil, goapalache.CheckOptions{MaxSteps: 1, Invariants: []string{tc.property}})
		if err != nil || symbolic.Outcome != tc.symbolic {
			t.Fatalf("assignment IR %s semantics: %+v, %v", tc.property, symbolic, err)
		}
	}
	afterChecks, err := json.Marshal(compiled.MainModule)
	if err != nil || !bytes.Equal(before, afterChecks) {
		t.Fatal("Apalache foldr adaptation mutated the caller's compilation", err)
	}
}
