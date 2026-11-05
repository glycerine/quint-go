package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamAssignments(t *testing.T) { testStateTransitions(t, "runtime_assignment.json") }
func TestUpstreamActionSequence(t *testing.T) {
	testStateTransitions(t, "runtime_action_sequence.json")
}
func TestUpstreamActionChoice(t *testing.T)      { testStateTransitions(t, "runtime_action_any.json") }
func TestUpstreamActionConjunction(t *testing.T) { testStateTransitions(t, "runtime_action_all.json") }
func testStateTransitions(t *testing.T, filename string) {
	t.Helper()
	t.Parallel()
	var fixtures []struct {
		Trace     *[]Expr
		Operation string
		RNG       *string
		Input     *Expr
		Output    *Expr
		Error     *Diagnostic
		Record    Expr
	}
	data, err := os.ReadFile("testdata/parser/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	trace := &RuntimeTrace{}
	rng := NewRNG(Integer{})
	storage := NewVariableStorage()
	storage.Register("x", "x")
	storage.Register("y", "y")
	for index, f := range fixtures {
		if f.Operation == "shift" {
			storage.Shift()
		} else {
			got, err := evaluateExpression(context.Background(), *f.Input, nil, storage, rng, trace)
			if f.Error != nil {
				var d Diagnostic
				if !errors.As(err, &d) || d.Code != f.Error.Code || d.Message != f.Error.Message || d.Reference == nil {
					t.Fatal(index, err, f.Error)
				}
			} else {
				if err != nil {
					t.Fatal(index, err)
				}
				want, err := NewRuntimeValue(*f.Output)
				if err != nil || !got.Equal(want) {
					t.Fatal(index, got.String(), want.String(), err)
				}
			}
		}
		if f.Trace != nil {
			got := mustStates(trace)
			if len(got) != len(*f.Trace) {
				t.Fatal(index, "trace length", len(got), len(*f.Trace))
			}
			for i, expression := range *f.Trace {
				want, err := NewRuntimeValue(expression)
				if err != nil || !got[i].Equal(want) {
					t.Fatal(index, "trace state", i, got[i].String(), want.String(), err)
				}
			}
		}
		if f.RNG != nil && rng.State().String() != *f.RNG {
			t.Fatal(index, "RNG counter", rng.State().String(), *f.RNG)
		}
		got, err := storage.Record()
		want, e := NewRuntimeValue(f.Record)
		if err != nil || e != nil || !got.Equal(want) {
			t.Fatal(index, got.String(), want.String(), err)
		}
	}
}
func TestAssignmentValidationAndFailedRHS(t *testing.T) {
	storage := NewVariableStorage()
	current, next := storage.Register("id", "x")
	initial, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	current.Value = initial
	next.Value = initial
	e, err := ParseExpression("x'=1/0")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EvaluateStateExpression(context.Background(), e, storage); err == nil || !next.Value.Equal(initial) || !current.Value.Equal(initial) {
		t.Fatal("failed RHS changed register", err)
	}
	for _, e := range []Expr{{Kind: "app", Opcode: "assign"}, {Kind: "app", Opcode: "assign", Args: []Expr{{Kind: "int"}, {Kind: "int"}}}, {Kind: "app", Opcode: "assign", Args: []Expr{{Kind: "name", Name: "missing"}, {Kind: "int"}}}} {
		if _, err := EvaluateStateExpression(context.Background(), e, storage); err == nil {
			t.Fatal("invalid assignment accepted")
		}
	}
	if _, err = EvaluateStateExpression(context.Background(), e, nil); err == nil {
		t.Fatal("nil storage accepted")
	}
	if _, err = EvaluatePureExpression(context.Background(), e, nil); err == nil {
		t.Fatal("pure evaluator accepted assignment")
	}
	shadow, err := ParseExpression("{ val x=1; x'=2 }")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = EvaluateStateExpression(context.Background(), shadow, storage); err == nil {
		t.Fatal("assignment to local accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = EvaluateStateExpression(ctx, e, storage); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
