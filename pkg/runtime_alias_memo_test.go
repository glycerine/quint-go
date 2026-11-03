package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamTransportedAliasCompilation(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name   string
		Module Module
		Steps  []struct {
			Kind, Name, ActionTaken string
			Output                  *Expr
			Error                   *Diagnostic
			Events                  []runtimeActionLabelEvent
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_alias_memo.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			evaluator, err := NewModuleEvaluator(fixture.Module, nil, true)
			if err != nil {
				t.Fatal(err)
			}
			listener := &executionRecorderSourceListener{TraceRecorder: NewTraceRecorder(3, evaluator.RNG, 2), t: t, events: []executionRecorderEvent{}}
			ctx := WithExecutionListener(context.Background(), listener)
			for index, step := range fixture.Steps {
				switch step.Kind {
				case "clear":
					evaluator.Storage.ClearMetadata()
				case "prepare":
					if err := evaluator.PrepareDefinition(ctx, step.Name); err != nil {
						t.Fatal(index, step.Name, err)
					}
				case "evaluate":
					value, err := evaluator.Evaluate(ctx, step.Name, nil)
					if step.Error != nil {
						var diagnostic Diagnostic
						if !errors.As(err, &diagnostic) || diagnostic.Code != step.Error.Code || diagnostic.Message != step.Error.Message || !reflect.DeepEqual(diagnostic.Reference, step.Error.Reference) {
							t.Fatal(index, step.Name, err, step.Error)
						}
					} else {
						if err != nil || step.Output == nil {
							t.Fatal(index, step.Name, value.String(), err)
						}
						got, err := SerializeRuntimeValueContext(ctx, value, evaluator.ObjectHashState())
						if err != nil || !reflect.DeepEqual(got, *step.Output) {
							t.Fatal(index, step.Name, got, step.Output, err)
						}
					}
				default:
					t.Fatal("unknown operation", step.Kind)
				}
				events := make([]runtimeActionLabelEvent, len(listener.events))
				for i, event := range listener.events {
					events[i] = runtimeActionLabelEvent{Kind: event.Kind, App: event.App}
				}
				got, _ := json.Marshal(events)
				want, _ := json.Marshal(step.Events)
				if string(got) != string(want) {
					t.Fatalf("step %d %s: got %s, want %s", index, step.Name, got, want)
				}
				listener.events = listener.events[:0]
				if evaluator.Storage.ActionTaken != step.ActionTaken {
					t.Fatal(index, step.Name, evaluator.Storage.ActionTaken, step.ActionTaken)
				}
			}
		})
	}
}
