package quint

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamRuntimeInstanceValues(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source, Module string
		RuntimeBoundary      bool
		AnalysisDiagnostics  []Diagnostic
		Steps                []struct {
			Kind, Name, Save, Left, Right string
			Arguments                     []int64
			Closure                       bool
			Output                        *Expr
			Error                         *Diagnostic
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_instance_values.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			ids := NewIDGenerator()
			modules, err := parseModulesWithIDs(fixture.Source, ids)
			if err != nil {
				t.Fatal(err)
			}
			var evaluator *ModuleEvaluator
			if fixture.RuntimeBoundary {
				linked, resolveError := ResolveLinkedModules(modules)
				if resolveError != nil || len(linked.Errors) > 0 {
					t.Fatal(resolveError, linked.Errors)
				}
				analysis, analyzeError := AnalyzeModules(modules, linked.Table)
				if analyzeError != nil || len(analysis.Diagnostics) == 0 || len(fixture.AnalysisDiagnostics) == 0 {
					t.Fatal("runtime-only factory boundary changed", analyzeError, analysis.Diagnostics)
				}
				flat, flattenError := FlattenModules(modules, linked.Table, ids, nil, AnalysisOutput{})
				if flattenError != nil {
					t.Fatal(flattenError)
				}
				for _, module := range flat.Modules {
					if module.Name == fixture.Module {
						evaluator, err = NewModuleEvaluator(module, nil, false)
					}
				}
			} else {
				evaluator, err = NewProjectEvaluator(context.Background(), modules, ids, fixture.Module, nil, false)
			}
			if err != nil {
				t.Fatal(err)
			}
			if evaluator == nil {
				t.Fatal("fixture module not found")
			}
			saved := map[string]RuntimeValue{}
			for index, step := range fixture.Steps {
				var value RuntimeValue
				switch step.Kind {
				case "shift":
					if err := evaluator.Shift(); err != nil {
						t.Fatal(err)
					}
					continue
				case "evaluate", "capture":
					value, err = evaluator.Evaluate(context.Background(), step.Name, nil)
				case "call":
					arguments := make([]RuntimeValue, len(step.Arguments))
					for i, argument := range step.Arguments {
						literal, parseError := ParseInteger(fmt.Sprint(argument))
						if parseError != nil {
							t.Fatal(parseError)
						}
						arguments[i], err = NewRuntimeValue(Expr{Kind: "int", Int: literal})
						if err != nil {
							t.Fatal(err)
						}
					}
					value, err = saved[step.Name].Apply(context.Background(), arguments)
				case "same":
					value, err = NewRuntimeValue(Expr{Kind: "bool", Bool: sameRuntimeObject(saved[step.Left], saved[step.Right])})
				default:
					t.Fatal("unknown operation", step.Kind)
				}
				if step.Error != nil {
					var diagnostic Diagnostic
					if !errors.As(err, &diagnostic) || diagnostic.Code != step.Error.Code || diagnostic.Message != step.Error.Message || !reflect.DeepEqual(diagnostic.Reference, step.Error.Reference) {
						t.Fatalf("step %d %s: got %v, want %v", index, step.Kind, err, *step.Error)
					}
					continue
				}
				if err != nil {
					t.Fatalf("step %d %s: %v", index, step.Kind, err)
				}
				if step.Closure {
					if value.closure == nil {
						t.Fatalf("step %d: expected callable, got %s", index, value.String())
					}
					saved[step.Save] = value
					continue
				}
				got, serializeError := SerializeRuntimeValueContext(context.Background(), value, evaluator.ObjectHashState())
				if serializeError != nil || step.Output == nil || !reflect.DeepEqual(got, *step.Output) {
					t.Fatalf("step %d %s: got %s (%v), want %+v", index, step.Kind, value.String(), serializeError, step.Output)
				}
			}
		})
	}
}

func TestRuntimeInstanceSourceMetadataJSONBoundary(t *testing.T) {
	t.Parallel()
	source := "module P {const n:int def f(x)=n+x} module M {import P(n=1) as A import P(n=100) as B def both(f,g)=List(f(0),g(0)) val result=both(A::f,B::f)}"
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs(source, ids)
	if err != nil {
		t.Fatal(err)
	}
	original, err := NewProjectEvaluator(context.Background(), modules, ids, "M", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(original.module)
	if err != nil {
		t.Fatal(err)
	}
	var transported Module
	if err := json.Unmarshal(data, &transported); err != nil {
		t.Fatal(err)
	}
	standalone, err := NewModuleEvaluator(transported, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		evaluator *ModuleEvaluator
		want      string
	}{{original, "List(100, 100)"}, {standalone, "List(1, 100)"}} {
		value, err := check.evaluator.Evaluate(context.Background(), "result", nil)
		if err != nil || value.String() != check.want {
			t.Fatal(value.String(), check.want, err)
		}
	}
}

func TestRuntimeInstanceSourceOwnershipAndCancellation(t *testing.T) {
	t.Parallel()
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module P {const n:int def f(x)=n+x} module M {import P(n=1) as A import P(n=100) as B def both(f,g)=List(f(0),g(0)) val result=both(A::f,B::f) def saved=A::f}", ids)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewProjectEvaluator(context.Background(), modules, ids, "M", nil, false)
	if err != nil {
		t.Fatal(err)
	}
	for i := range modules {
		for j := range modules[i].Declarations {
			declaration := &modules[i].Declarations[j]
			if declaration.Expr != nil {
				*declaration.Expr = Expr{Kind: "bool", Bool: false}
			}
			for k := range declaration.Overrides {
				declaration.Overrides[k].Expr = Expr{Kind: "bool", Bool: false}
			}
		}
	}
	value, err := evaluator.Evaluate(context.Background(), "result", nil)
	if err != nil || value.String() != "List(100, 100)" {
		t.Fatal("source bodies retained caller-owned input", value.String(), err)
	}
	closure, err := evaluator.Evaluate(context.Background(), "saved", nil)
	if err != nil || closure.closure == nil {
		t.Fatal(closure.String(), err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := evaluator.Evaluate(canceled, "result", nil); !errors.Is(err, context.Canceled) {
		t.Fatal("cached source value ignored cancellation", err)
	}
	argument, err := NewRuntimeValue(Expr{Kind: "int"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := closure.Apply(canceled, []RuntimeValue{argument}); !errors.Is(err, context.Canceled) {
		t.Fatal("saved source closure ignored cancellation", err)
	}
	value, err = closure.Apply(context.Background(), []RuntimeValue{argument})
	if err != nil || value.String() != "1" {
		t.Fatal("canceled call changed source registers", value.String(), err)
	}
}
