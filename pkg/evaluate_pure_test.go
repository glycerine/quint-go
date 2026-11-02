package quint

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/glycerine/insdict"
	"os"
	"testing"
)

func TestUpstreamPureEvaluation(t *testing.T) { testPureEvaluationFixtures(t, "evaluate_pure.json") }
func TestUpstreamLocalOperatorEvaluation(t *testing.T) {
	testPureEvaluationFixtures(t, "runtime_operator.json")
}
func TestUpstreamLocalValueEvaluation(t *testing.T) {
	testPureEvaluationFixtures(t, "runtime_let.json")
}
func TestUpstreamMapEvaluation(t *testing.T) { testPureEvaluationFixtures(t, "runtime_map.json") }
func TestUpstreamSetEvaluation(t *testing.T) { testPureEvaluationFixtures(t, "runtime_set.json") }
func TestUpstreamSequenceEvaluation(t *testing.T) {
	testPureEvaluationFixtures(t, "runtime_sequence.json")
}
func testPureEvaluationFixtures(t *testing.T, filename string) {
	t.Helper()
	t.Parallel()
	var fixtures []struct {
		Source string
		Input  Expr
		Output *Expr
		Error  *Diagnostic
	}
	data, err := os.ReadFile("testdata/parser/" + filename)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Source, func(t *testing.T) {
			got, err := EvaluatePureExpression(context.Background(), f.Input, nil)
			if f.Error != nil {
				var d Diagnostic
				if !errors.As(err, &d) || d.Code != f.Error.Code || d.Message != f.Error.Message {
					t.Fatal(err, f.Error)
				}
				if d.Reference == nil {
					t.Fatal("missing source reference")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want, err := NewRuntimeValue(*f.Output)
			if err != nil || !got.Equal(want) {
				t.Fatal(got.String(), want.String(), err)
			}
		})
	}
}
func TestPureEvaluationBindingsCancellationAndErrors(t *testing.T) {
	bindings := insdict.NewDict[string, RuntimeValue]()
	n, _ := ParseInteger("42")
	value, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	bindings.Set("x", value)
	e, err := ParseExpression("x + 1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := EvaluatePureExpression(context.Background(), e, bindings)
	if err != nil || got.String() != "43" || bindings.Get("x").String() != "42" {
		t.Fatal(got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = EvaluatePureExpression(ctx, e, bindings); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, e := range []Expr{{Kind: "name", Name: "missing"}, {Kind: "lambda"}, {Kind: "app", Opcode: "ite"}, {Kind: "app", Opcode: "not"}, {Kind: "app", Opcode: "and", Args: []Expr{{Kind: "int"}}}, {Kind: "app", Opcode: "implies", Args: []Expr{{Kind: "bool", Bool: true}, {Kind: "int"}}}, {Kind: "app", Opcode: "not", Args: []Expr{{Kind: "int"}}}} {
		if _, err := EvaluatePureExpression(context.Background(), e, nil); err == nil {
			t.Fatal("invalid expression accepted", e)
		}
	}
}
