package quint

import (
	"context"
	"testing"
)

func TestActionConjunctionRollbackPreservesBaseline(t *testing.T) {
	storage := NewVariableStorageWithMetadata(true)
	current, next := storage.Register("id", "x")
	n, _ := ParseInteger("7")
	value, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	next.Value = value
	storage.ActionTaken = "baseline"
	storage.NondetPicks.Set("pick", value)
	expression, err := ParseExpression("all { x'=99, false, x'=100 }")
	if err != nil {
		t.Fatal(err)
	}
	result, err := EvaluateStateExpression(context.Background(), expression, storage)
	if err != nil || result.String() != "false" || !next.Value.Equal(value) || current.Value.Valid() {
		t.Fatal(result.String(), err)
	}
	if storage.ActionTaken != "baseline" || !storage.NondetPicks.Get("pick").Equal(value) {
		t.Fatal("metadata rollback changed baseline")
	}
	storage.Shift()
	if !current.Value.Equal(value) {
		t.Fatal("tentative write leaked")
	}
	if _, err := EvaluatePureExpression(context.Background(), expression, nil); err == nil {
		t.Fatal("pure evaluator accepted action")
	}
}
