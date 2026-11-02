package quint

import (
	"context"
	"testing"
)

func TestUpstreamExpect(t *testing.T) { testStateTransitions(t, "runtime_expect.json") }

func TestExpectContextAndMetadata(t *testing.T) {
	modules, err := ParseModules("module M { var x:int action init=x'=1 action check=(x'=2).expect(x==2) val read=x }")
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewModuleEvaluator(modules[0], nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = evaluator.Evaluate(context.Background(), "init", nil); err != nil {
		t.Fatal(err)
	}
	evaluator.Storage.Shift()
	evaluator.Storage.ClearMetadata()
	if _, err = evaluator.Evaluate(context.Background(), "check", nil); err != nil {
		t.Fatal(err)
	}
	if evaluator.Storage.ActionTaken != "check" {
		t.Fatal("expect did not recover action metadata")
	}
	evaluator.Storage.Shift()
	record, err := evaluator.Evaluate(context.Background(), "read", nil)
	if err != nil || record.String() != "2" {
		t.Fatal(record.String(), err)
	}
	expression := Expr{Kind: "app", Opcode: "expect", Args: []Expr{{Kind: "bool", Bool: true}, {Kind: "bool", Bool: true}}}
	if _, err = EvaluatePureExpression(context.Background(), expression, nil); err == nil {
		t.Fatal("pure expect accepted")
	}
	expression.Args = nil
	if _, err = EvaluateStateExpression(context.Background(), expression, evaluator.Storage); err == nil {
		t.Fatal("malformed expect accepted")
	}
}
