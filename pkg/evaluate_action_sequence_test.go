package quint

import (
	"context"
	"errors"
	"testing"
)

func TestActionSequenceCachesMetadataAndCancellation(t *testing.T) {
	storage := NewVariableStorageWithMetadata(true)
	current, next := storage.Register("x", "x")
	one, _ := ParseInteger("1")
	value, _ := NewRuntimeValue(Expr{Kind: "int", Int: one})
	current.Value = value
	storage.ActionTaken = "step"
	storage.NondetPicks.Set("p", value)
	expression, err := ParseExpression("(x'=x+1).then(x'=x+1)")
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvaluateStateExpression(context.Background(), expression, storage)
	if err != nil || result.String() != "true" || current.Value.String() != "2" || next.Value.String() != "3" || storage.ActionTaken != "" || storage.NondetPicks.Get("p").Valid() {
		t.Fatal(result.String(), err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	callbackCell, _ := storage.Register("callback", "callback")
	calls := 0
	callbackCell.Value = newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
		calls++
		cancel()
		return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	})
	expression, err = ParseExpression("100.reps(callback)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EvaluateStateExpression(ctx, expression, storage); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal(err, calls)
	}
	for _, expression := range []Expr{{Kind: "app", Opcode: "then"}, {Kind: "app", Opcode: "reps"}, {Kind: "app", Opcode: "reps", Args: []Expr{{Kind: "bool"}, {Kind: "bool"}}}, {Kind: "app", Opcode: "reps", Args: []Expr{{Kind: "int", Int: one}, {Kind: "bool"}}}} {
		if _, err := EvaluateStateExpression(context.Background(), expression, storage); err == nil {
			t.Fatal("invalid sequencing accepted")
		}
	}
	if _, err := EvaluatePureExpression(context.Background(), expression, nil); err == nil {
		t.Fatal("pure sequencing accepted")
	}
}
