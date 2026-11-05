package quint

import (
	"context"
	"testing"
)

func TestUpstreamSequenceTrace(t *testing.T) {
	testStateTransitions(t, "runtime_action_sequence_trace.json")
}
func TestUpstreamExpectTrace(t *testing.T) { testStateTransitions(t, "runtime_expect_trace.json") }
func TestRuntimeTraceOwnershipAndModuleShift(t *testing.T) {
	value, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	input := []RuntimeValue{value}
	var trace RuntimeTrace
	trace.Reset(input...)
	input[0] = RuntimeValue{}
	states := mustStates(&trace)
	states[0] = RuntimeValue{}
	if !mustStates(&trace)[0].Equal(value) {
		t.Fatal("trace slice aliased")
	}
	trace.DropLast()
	trace.DropLast()
	if len(mustStates(&trace)) != 0 {
		t.Fatal("drop empty")
	}
	modules, err := ParseModules("module M { var x:int action init=x'=1 action step=x'=x+1 }")
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewModuleEvaluator(modules[0], nil, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"init", "step"} {
		if _, err = evaluator.Evaluate(context.Background(), name, nil); err != nil {
			t.Fatal(err)
		}
		if err = evaluator.Shift(); err != nil {
			t.Fatal(err)
		}
	}
	states = mustStates(&evaluator.Trace)
	if len(states) != 2 {
		t.Fatal(len(states))
	}
	for i, state := range states {
		n, _ := ParseInteger([]string{"1", "2"}[i])
		want, err := NewRuntimeValue(Expr{Kind: "app", Opcode: "Rec", Args: []Expr{
			{Kind: "str", Str: "x"}, {Kind: "int", Int: n},
			{Kind: "str", Str: "mbt::actionTaken"}, {Kind: "str", Str: []string{"init", "step"}[i]},
			{Kind: "str", Str: "mbt::nondetPicks"}, {Kind: "app", Opcode: "Rec"},
		}})
		if err != nil || !state.Equal(want) {
			t.Fatal(state.String(), want.String(), err)
		}
	}
	if evaluator.Storage.ActionTaken != "" {
		t.Fatal("metadata not cleared")
	}
	evaluator.Trace.Reset()
	if len(mustStates(&evaluator.Trace)) != 0 {
		t.Fatal("reset")
	}
}
