package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamStatefulProjects(t *testing.T) {
	t.Parallel()
	testUpstreamStatefulProjects(t, "testdata/parser/runtime_state_project.json", false)
}

func TestUpstreamSourceStatefulProjects(t *testing.T) {
	t.Parallel()
	testUpstreamStatefulProjects(t, "testdata/parser/runtime_state_project_source.json", true)
}

func testUpstreamStatefulProjects(t *testing.T, fixtureFile string, sourceRuntime bool) {
	t.Helper()
	var fixtures []struct {
		Name   string
		Source string
		Steps  []struct {
			Operation string
			Output    *Expr
			Error     *Diagnostic
			Record    Expr
			RNG       string
		}
	}
	data, err := os.ReadFile(fixtureFile)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			t.Parallel()
			ids := NewIDGenerator()
			modules, err := parseModulesWithIDs(f.Source, ids)
			if err != nil {
				t.Fatal(err)
			}
			before, _ := json.Marshal(modules)
			next := ids.next.String()
			var evaluator *ModuleEvaluator
			if sourceRuntime {
				evaluator, err = NewProjectEvaluator(context.Background(), modules, ids, "M", NewRNG(Integer{}), true)
			} else {
				// This original oracle explicitly flattens TypeScript IR before
				// evaluation. Transport drops private source provenance, keeping
				// that boundary distinct from source project runtime semantics.
				var flat Module
				flat, err = prepareEvaluationProject(context.Background(), modules, ids, "M")
				if err == nil {
					var data []byte
					data, err = json.Marshal(flat)
					if err == nil {
						err = json.Unmarshal(data, &flat)
					}
					if err == nil {
						evaluator, err = NewModuleEvaluator(flat, NewRNG(Integer{}), true)
					}
				}
			}
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
			after, _ := json.Marshal(modules)
			if string(before) != string(after) || ids.next.String() != next {
				t.Fatal("module mutated")
			}
		})
	}
}
func TestStatefulProjectEvaluationValidation(t *testing.T) {
	for _, source := range []string{
		"module M { val result=1+true }",
		"module M { val result=missing }",
		"module M { import Missing.* val result=1 }",
		"module M { val a=b val b=a val result=a }",
	} {
		ids := NewIDGenerator()
		modules, err := parseModulesWithIDs(source, ids)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = NewProjectEvaluator(context.Background(), modules, ids, "M", nil, false); err == nil {
			t.Fatal("invalid project accepted", source)
		}
	}
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module M { val result=42 }", ids)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewProjectEvaluator(context.Background(), modules, nil, "M", nil, false); err == nil {
		t.Fatal("nil generator accepted")
	}
	if _, err := NewProjectEvaluator(context.Background(), modules, ids, "Missing", nil, false); err == nil {
		t.Fatal("missing main accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := NewProjectEvaluator(ctx, modules, ids, "M", nil, false); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
