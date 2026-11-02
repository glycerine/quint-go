package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamStatefulModules(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name   string
		Module Module
		Steps  []struct {
			Operation string
			Output    *Expr
			Error     *Diagnostic
			Record    Expr
			RNG       string
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_state_module.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			before, _ := json.Marshal(f.Module)
			evaluator, err := NewModuleEvaluator(f.Module, NewRNG(Integer{}), true)
			if err != nil {
				t.Fatal(err)
			}
			for index, step := range f.Steps {
				if step.Operation == "shift" {
					evaluator.Storage.Shift()
					evaluator.Storage.ClearMetadata()
				} else {
					got, err := evaluator.Evaluate(context.Background(), step.Operation, nil)
					if step.Error != nil {
						var d Diagnostic
						if !errors.As(err, &d) || d.Code != step.Error.Code || d.Message != step.Error.Message {
							t.Fatal(index, err, step.Error)
						}
					} else {
						if err != nil {
							t.Fatal(index, err)
						}
						want, err := NewRuntimeValue(*step.Output)
						if err != nil || !got.Equal(want) {
							t.Fatal(index, got.String(), want.String(), err)
						}
					}
				}
				got, err := evaluator.Storage.Record()
				want, e := NewRuntimeValue(step.Record)
				if err != nil || e != nil || !got.Equal(want) || evaluator.RNG.State().String() != step.RNG {
					t.Fatal(index, got.String(), want.String(), err, evaluator.RNG.State().String(), step.RNG)
				}
			}
			after, _ := json.Marshal(f.Module)
			if string(before) != string(after) {
				t.Fatal("module mutated")
			}
		})
	}
}
func TestModuleEvaluatorOwnershipCallsAndErrors(t *testing.T) {
	modules, err := ParseModules("module M { var x:int action init=x'=1 action add(n)=x'=x+n val read=x }")
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewModuleEvaluator(modules[0], nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := range modules[0].Declarations {
		if modules[0].Declarations[i].Name == "init" {
			modules[0].Declarations[i].Expr.Args[1] = Expr{Kind: "bool", Bool: false}
		}
	}
	if _, err = evaluator.Evaluate(context.Background(), "init", nil); err != nil {
		t.Fatal(err)
	}
	evaluator.Storage.Shift()
	n, _ := ParseInteger("2")
	arg, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	if _, err = evaluator.Evaluate(context.Background(), "add", []RuntimeValue{arg}); err != nil {
		t.Fatal(err)
	}
	evaluator.Storage.Shift()
	got, err := evaluator.Evaluate(context.Background(), "read", nil)
	if err != nil || got.String() != "3" {
		t.Fatal(got.String(), err)
	}
	if _, err = evaluator.Evaluate(context.Background(), "missing", nil); err == nil {
		t.Fatal("missing definition accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = evaluator.Evaluate(ctx, "read", nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, source := range []string{"module M { import A.* }", "module M { val x=missing }", "module M { val a=b val b=a }"} {
		modules, err := ParseModules(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = NewModuleEvaluator(modules[0], nil, false); err == nil {
			t.Fatal("invalid module accepted", source)
		}
	}
}
