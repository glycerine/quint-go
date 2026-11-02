package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strconv"
	"testing"
)

func TestUpstreamCompiledClosureLifetime(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		ExpressionCases []struct {
			Name        string
			Input       Expr
			Evaluations []struct {
				Output *Expr
				Error  *Diagnostic
			}
		}
		Sessions []struct {
			Name                                               string
			Input                                              Expr
			RepeatedBuildSameObject, NewBuilderDifferentObject bool
			Steps                                              []struct {
				Kind                           string
				Argument                       int
				Child                          int
				Output                         *Expr
				Error                          *Diagnostic
				SameAsFirst, DirectEqualsFirst bool
			}
		}
		ModuleEvaluations struct {
			Module      Module
			Evaluations []struct {
				Name          string
				Output        *Expr
				SameAsFirst   bool
				InvokeWithOne struct {
					Output *Expr
					Error  *Diagnostic
				}
			}
			SeparateEvaluator struct {
				DifferentClosure                   bool
				NewInvokeWithOne, OldInvokeWithOne struct {
					Output *Expr
					Error  *Diagnostic
				}
			}
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_closure_lifetime.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	compare := func(t *testing.T, got RuntimeValue, err error, output *Expr, wantErr *Diagnostic) {
		t.Helper()
		if wantErr != nil {
			var d Diagnostic
			if !errors.As(err, &d) || d.Code != wantErr.Code || d.Message != wantErr.Message {
				t.Fatal(err, wantErr)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if output == nil || !reflect.DeepEqual(got.Expression(), *output) {
			a, _ := json.Marshal(got.Expression())
			b, _ := json.Marshal(output)
			t.Fatalf("got %s\nwant %s", a, b)
		}
	}
	integer := func(n int) RuntimeValue {
		integer, err := ParseInteger(strconv.Itoa(n))
		if err != nil {
			t.Fatal(err)
		}
		value, err := NewRuntimeValue(Expr{Kind: "int", Int: integer})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, scenario := range fixtures.ExpressionCases {
		t.Run(scenario.Name, func(t *testing.T) {
			expression := copyRuntimeExpression(scenario.Input)
			session := newRuntimeEvaluation(nil, nil, nil, nil, nil)
			for _, run := range scenario.Evaluations {
				got, err := session.evaluateOwned(ctx, expression)
				compare(t, got, err, run.Output, run.Error)
			}
		})
	}
	for _, scenario := range fixtures.Sessions {
		t.Run(scenario.Name, func(t *testing.T) {
			expression := copyRuntimeExpression(scenario.Input)
			storage := NewVariableStorage()
			session := newRuntimeEvaluation(nil, storage, nil, nil, nil)
			factory, err := session.evaluateOwned(ctx, expression)
			if err != nil {
				t.Fatal(err)
			}
			repeat, err := session.evaluate(ctx, expression)
			if err != nil || sameRuntimeObject(factory, repeat) != scenario.RepeatedBuildSameObject {
				t.Fatal("compiled allocation changed", err)
			}
			separate, err := newRuntimeEvaluation(nil, nil, nil, nil, nil).evaluateOwned(ctx, copyRuntimeExpression(expression))
			if err != nil || (!sameRuntimeObject(factory, separate)) != scenario.NewBuilderDifferentObject {
				t.Fatal("execution allocation leaked", err)
			}
			children := []RuntimeValue{}
			for _, step := range scenario.Steps {
				switch step.Kind {
				case "factory":
					got, err := factory.Apply(ctx, []RuntimeValue{integer(step.Argument)})
					compare(t, got, err, step.Output, step.Error)
					values, err := runtimeChildren(got)
					if err != nil {
						t.Fatal(err)
					}
					children = append(children, values[0])
					if sameRuntimeObject(values[0], children[0]) != step.SameAsFirst || values[0].Equal(children[0]) != step.DirectEqualsFirst {
						t.Fatal("factory closure identity/comparison")
					}
				case "callChild":
					got, err := children[step.Child].Apply(ctx, []RuntimeValue{integer(step.Argument)})
					compare(t, got, err, step.Output, step.Error)
				case "reset":
					storage.Reset()
				default:
					t.Fatal(step.Kind)
				}
			}
		})
	}
	t.Run("persistent_module_evaluations", func(t *testing.T) {
		scenario := fixtures.ModuleEvaluations
		evaluator, err := NewModuleEvaluator(scenario.Module, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		var first RuntimeValue
		for _, step := range scenario.Evaluations {
			got, err := evaluator.Evaluate(ctx, step.Name, nil)
			compare(t, got, err, step.Output, nil)
			if !first.Valid() {
				first = got
			}
			if sameRuntimeObject(got, first) != step.SameAsFirst {
				t.Fatal("module lambda allocation changed")
			}
			value, err := got.Apply(ctx, []RuntimeValue{integer(1)})
			compare(t, value, err, step.InvokeWithOne.Output, step.InvokeWithOne.Error)
		}
		separate, err := NewModuleEvaluator(scenario.Module, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		fresh, err := separate.Evaluate(ctx, scenario.Evaluations[0].Name, nil)
		if err != nil || (!sameRuntimeObject(first, fresh)) != scenario.SeparateEvaluator.DifferentClosure {
			t.Fatal("module lifetime leaked", err)
		}
		value, err := fresh.Apply(ctx, []RuntimeValue{integer(1)})
		compare(t, value, err, scenario.SeparateEvaluator.NewInvokeWithOne.Output, scenario.SeparateEvaluator.NewInvokeWithOne.Error)
		value, err = first.Apply(ctx, []RuntimeValue{integer(1)})
		compare(t, value, err, scenario.SeparateEvaluator.OldInvokeWithOne.Output, scenario.SeparateEvaluator.OldInvokeWithOne.Error)
	})
}

func TestCompiledModuleUninitializedConstant(t *testing.T) {
	parsed, err := ParseModules("module M { const n:int val selected=n val unused=n+1 }")
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewModuleEvaluator(parsed[0], nil, false)
	if err != nil {
		t.Fatal(err)
	}
	_, err = evaluator.Evaluate(context.Background(), "selected", nil)
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != "QNT500" || diagnostic.Message != "Uninitialized const n. Use: import <moduleName>(n=<value>).*" || diagnostic.Reference == nil {
		t.Fatal("uninitialized constant", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = evaluator.Evaluate(ctx, "selected", nil); !errors.Is(err, context.Canceled) {
		t.Fatal("cached diagnostic suppresses caller cancellation", err)
	}
}
