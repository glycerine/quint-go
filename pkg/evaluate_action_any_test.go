package quint

import (
	"context"
	"testing"
)

func TestActionChoiceRollbackAndMetadata(t *testing.T) {
	storage := NewVariableStorageWithMetadata(false)
	current, next := storage.Register("id", "x")
	n, _ := ParseInteger("7")
	value, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	next.Value = value
	storage.ActionTaken = "baseline"
	storage.NondetPicks.Set("p", value)
	rng := NewRNG(Integer{})
	expression, err := ParseExpression("any { all { x'=99, false }, false }")
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvaluateStateExpressionWithRNG(context.Background(), expression, storage, rng)
	if err != nil || result.String() != "false" || !next.Value.Equal(value) || current.Value.Valid() || storage.ActionTaken != "baseline" || !storage.NondetPicks.Get("p").Equal(value) {
		t.Fatal(result.String(), err)
	}
	expression, err = ParseExpression("any { x'=8 }")
	if err != nil {
		t.Fatal(err)
	}
	before := rng.State()
	result, err = EvaluateStateExpressionWithRNG(context.Background(), expression, storage, rng)
	if err != nil || result.String() != "true" || next.Value.String() != "8" || rng.State() != before || storage.ActionTaken != "" || storage.NondetPicks.Get("p").Valid() {
		t.Fatal(result.String(), err)
	}
	if _, err := EvaluateStateExpression(context.Background(), expression, storage); err == nil {
		t.Fatal("missing RNG accepted")
	}
	if _, err := EvaluateStateExpressionWithRNG(context.Background(), expression, nil, rng); err == nil {
		t.Fatal("missing storage accepted")
	}
	if _, err := EvaluatePureExpression(context.Background(), expression, nil); err == nil {
		t.Fatal("pure action choice accepted")
	}
}
