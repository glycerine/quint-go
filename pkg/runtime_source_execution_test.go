package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamRuntimeSourceContextExecution(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source, Module string
		Steps                []struct {
			Kind, Name, RNG string
			Output          *Expr
			Error           *Diagnostic
			Record          Expr
			Registration    []runtimeVariableRegistration
			Names           []string
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_instance_execution.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			ids := NewIDGenerator()
			modules, err := parseModulesWithIDs(fixture.Source, ids)
			if err != nil {
				t.Fatal(err)
			}
			evaluator, err := NewProjectEvaluator(context.Background(), modules, ids, fixture.Module, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			for index, step := range fixture.Steps {
				switch step.Kind {
				case "prepare":
					if err := evaluator.PrepareDefinition(context.Background(), step.Name); err != nil {
						t.Fatal(index, step.Kind, err)
					}
				case "shift":
					if err := evaluator.Shift(); err != nil {
						t.Fatal(index, step.Kind, err)
					}
				case "evaluate":
					got, err := evaluator.Evaluate(context.Background(), step.Name, nil)
					if step.Error != nil {
						var diagnostic Diagnostic
						if !errors.As(err, &diagnostic) || diagnostic.Code != step.Error.Code || diagnostic.Message != step.Error.Message {
							t.Fatal(index, step.Name, err, step.Error)
						}
					} else {
						if err != nil || step.Output == nil {
							t.Fatal(index, step.Name, got.String(), err)
						}
						want, err := NewRuntimeValue(*step.Output)
						if err != nil || !got.Equal(want) {
							t.Fatal(index, step.Name, got.String(), want.String(), err)
						}
					}
				default:
					t.Fatal("unknown fixture operation", step.Kind)
				}
				assertRuntimeVariables(t, evaluator.Storage, runtimeVariableSnapshot{Registration: step.Registration, Names: step.Names})
				record, err := evaluator.Storage.Record()
				want, wantError := NewRuntimeValue(step.Record)
				if err != nil || wantError != nil || !record.Equal(want) {
					t.Fatal(index, step.Kind, record.String(), want.String(), err, wantError)
				}
				if evaluator.RNG.State().String() != step.RNG {
					t.Fatal(index, "source context changed RNG")
				}
			}
		})
	}
}

func TestUpstreamSavedExportAliasContextHistories(t *testing.T) {
	t.Parallel()
	type snapshot struct {
		Name          string
		Registrations []runtimeVariableRegistration
	}
	var fixture struct {
		Source, Main string
		Upstream     struct {
			Isolated   []snapshot
			Cumulative []struct {
				Order []string
				Steps []snapshot
			}
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_variable_context_pending.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	newEvaluator := func(t *testing.T) *ModuleEvaluator {
		t.Helper()
		ids := NewIDGenerator()
		modules, err := parseModulesWithIDs(fixture.Source, ids)
		if err != nil {
			t.Fatal(err)
		}
		// This saved witness exercises the source Builder directly, before
		// frontend module sorting. Follow that same boundary: exports do not
		// create sorting edges in the reference frontend, and these short
		// modules otherwise land in its lexical-ID ordering corner case.
		linked, err := ResolveLinkedModules(modules)
		if err != nil || len(linked.Errors) != 0 {
			t.Fatal(err, linked.Errors)
		}
		analysis, err := AnalyzeModules(modules, linked.Table)
		if err != nil || len(analysis.Diagnostics) != 0 {
			t.Fatal(err, analysis.Diagnostics)
		}
		flat, err := FlattenModules(analysis.Modules, linked.Table, ids, nil, AnalysisOutput{Types: analysis.Types, Effects: analysis.Effects, Modes: analysis.Modes})
		if err != nil {
			t.Fatal(err)
		}
		var main Module
		for _, module := range flat.Modules {
			if module.Name == fixture.Main {
				main = module
			}
		}
		evaluator, err := NewModuleEvaluator(main, nil, false)
		if err != nil {
			t.Fatal(err)
		}
		return evaluator
	}
	check := func(t *testing.T, evaluator *ModuleEvaluator, step snapshot) {
		t.Helper()
		if err := evaluator.PrepareDefinition(context.Background(), step.Name); err != nil {
			t.Fatal(err)
		}
		names := make([]string, len(step.Registrations))
		for i, registration := range step.Registrations {
			names[i] = registration.Name
		}
		assertRuntimeVariables(t, evaluator.Storage, runtimeVariableSnapshot{Registration: step.Registrations, Names: names})
		_, err := evaluator.Evaluate(context.Background(), step.Name, nil)
		var diagnostic Diagnostic
		if !errors.As(err, &diagnostic) || diagnostic.Code != "QNT502" || diagnostic.Message != "Variable "+names[0]+" not set" {
			t.Fatal("source read captured the wrong instance context", err)
		}
	}
	for _, isolated := range fixture.Upstream.Isolated {
		t.Run("isolated_"+isolated.Name, func(t *testing.T) { check(t, newEvaluator(t), isolated) })
	}
	for _, sequence := range fixture.Upstream.Cumulative {
		t.Run("first_"+sequence.Order[0], func(t *testing.T) {
			evaluator := newEvaluator(t)
			for _, step := range sequence.Steps {
				check(t, evaluator, step)
			}
		})
	}
}
