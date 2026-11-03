package quint

import (
	"context"
	"encoding/json"
	"os"
	"testing"
)

type runtimeActionLabelEvent struct {
	Kind string
	App  Expr
}

type runtimeActionLabelStep struct {
	Name        string
	Output      *Expr
	Events      []runtimeActionLabelEvent
	ActionTaken string
}

func TestUpstreamRuntimeActionLabels(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source       string
		Module             Module
		Steps, Transported []runtimeActionLabelStep
	}
	data, err := os.ReadFile("testdata/parser/runtime_action_labels.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		for _, transported := range []bool{false, true} {
			name := fixture.Name + "/source"
			if transported {
				name = fixture.Name + "/transported"
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				var evaluator *ModuleEvaluator
				var err error
				steps := fixture.Steps
				if transported {
					evaluator, err = NewModuleEvaluator(fixture.Module, nil, true)
					steps = fixture.Transported
				} else {
					ids := NewIDGenerator()
					modules, parseError := parseModulesWithIDs(fixture.Source, ids)
					if parseError != nil {
						t.Fatal(parseError)
					}
					evaluator, err = NewProjectEvaluator(context.Background(), modules, ids, "M", nil, true)
				}
				if err != nil {
					t.Fatal(err)
				}
				listener := &executionRecorderSourceListener{TraceRecorder: NewTraceRecorder(3, evaluator.RNG, 2), t: t, events: []executionRecorderEvent{}}
				ctx := WithExecutionListener(context.Background(), listener)
				for _, step := range steps {
					if step.Name == "shift" {
						if err := evaluator.Shift(); err != nil {
							t.Fatal(err)
						}
						continue
					}
					request := step.Name
					_, found := evaluator.definitions.Get2(request)
					if !transported && !found {
						// Instance declarations in the public flat evaluator retain
						// their containing module prefix; source builder apps do not.
						request = evaluator.module.Name + "::" + request
					}
					result, err := evaluator.Evaluate(ctx, request, nil)
					if err != nil || step.Output == nil {
						t.Fatal(step.Name, result, err)
					}
					want, err := NewRuntimeValue(*step.Output)
					if err != nil || !result.Equal(want) {
						t.Fatal(step.Name, result.String(), want.String(), err)
					}
					events := make([]runtimeActionLabelEvent, 0, len(listener.events))
					for _, event := range listener.events {
						events = append(events, runtimeActionLabelEvent{Kind: event.Kind, App: event.App})
					}
					got, _ := json.Marshal(events)
					wantEvents, _ := json.Marshal(step.Events)
					if string(got) != string(wantEvents) {
						t.Fatalf("%s: apps %s\nwant %s", step.Name, got, wantEvents)
					}
					listener.events = listener.events[:0]
					// Transported IR deliberately retains its flattened declaration
					// names for both frames and the existing action metadata API.
					if transported && evaluator.Storage.ActionTaken != step.ActionTaken {
						t.Fatal(step.Name, evaluator.Storage.ActionTaken, step.ActionTaken)
					}
				}
			})
		}
	}
}

func TestRuntimeActionLabelFallbackAndSnapshot(t *testing.T) {
	t.Parallel()
	modules, err := ParseModules("module M {action Qualified::step=true}")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := newRuntimeSourceGraph(modules)
	if err != nil {
		t.Fatal(err)
	}
	declaration := sourceRuntimeDeclaration(modules[0].Declarations[0], ID{})
	id, err := ParseID("100")
	if err != nil {
		t.Fatal(err)
	}
	definition := OperatorDef{ID: id, Name: "Imported::Qualified::step", Qualifier: Action, Expr: declaration.Expr}
	preparation := newRuntimeSourcePreparation(&runtimeSourceModule{graph: graph, name: "M"})
	app := preparation.ActionApp(definition)
	if app.Opcode != "Qualified::step" || app.ID != modules[0].Declarations[0].ID {
		t.Fatal(app)
	}
	preparation.UpdateSource(nil)
	if app.Opcode != "Qualified::step" {
		t.Fatal("captured app changed after source replacement")
	}
	for _, candidate := range []*runtimeSourcePreparation{nil, preparation, newRuntimeSourcePreparation(&runtimeSourceModule{graph: &runtimeSourceGraph{}})} {
		if fallback := candidate.ActionApp(definition); fallback.Opcode != definition.Name || fallback.ID != definition.ID {
			t.Fatal(fallback)
		}
	}
	preparation.UpdateSource(&runtimeSourceModule{graph: graph, name: "M"})
	definition.Expr = &Expr{Kind: "bool", Bool: true}
	if fallback := preparation.ActionApp(definition); fallback.Opcode != definition.Name {
		t.Fatal(fallback)
	}
	definition.Expr = nil
	if fallback := preparation.ActionApp(definition); fallback.Opcode != definition.Name {
		t.Fatal(fallback)
	}
}

func TestRuntimeActionLabelPreservesQualifiedActionMetadata(t *testing.T) {
	t.Parallel()
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module A {action start=true} module M {import A as C action keep=C::start}", ids)
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewProjectEvaluator(context.Background(), modules, ids, "M", nil, true)
	if err != nil {
		t.Fatal(err)
	}
	listener := &executionRecorderSourceListener{TraceRecorder: NewTraceRecorder(3, evaluator.RNG, 2), t: t, events: []executionRecorderEvent{}}
	if _, err := evaluator.Evaluate(WithExecutionListener(context.Background(), listener), "C::start", nil); err != nil {
		t.Fatal(err)
	}
	if evaluator.Storage.ActionTaken != "C::start" {
		t.Fatal(evaluator.Storage.ActionTaken)
	}
	if len(listener.events) != 2 || listener.events[0].App.Opcode != "start" {
		t.Fatal(listener.events)
	}
}

func BenchmarkRuntimeActionLabelFlattening(b *testing.B) {
	for _, actions := range []bool{false, true} {
		name, qualifier := "pure_values", "pure val"
		if actions {
			name, qualifier = "source_actions", "action"
		}
		b.Run(name, func(b *testing.B) {
			ids := NewIDGenerator()
			modules, err := parseModulesWithIDs("module A {"+qualifier+" start=true} module M {import A as C "+qualifier+" keep=C::start}", ids)
			if err != nil {
				b.Fatal(err)
			}
			linked, err := ResolveLinkedModules(modules)
			if err != nil || len(linked.Errors) != 0 {
				b.Fatal(err, linked.Errors)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				generator := NewIDGenerator()
				generator.next.Set(&ids.next)
				if _, err := FlattenModules(modules, linked.Table, generator, nil, AnalysisOutput{}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
