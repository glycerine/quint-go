package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

// These histories focus on incomplete argument frames, nested callbacks, and
// failed stateful combinators. Events and final retained trees both come from
// the actual upstream Builder and TraceRecorder, including their source IDs.
func TestUpstreamRuntimeListenerAudit(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source, Mode, Root, Seed string
		Module                         Module
		Input                          Expr
		Verbosity, Capacity            int
		Output                         *Expr
		Error                          *Diagnostic
		Events                         []executionRecorderEvent
		Final                          executionRecorderSnapshot
	}
	data, err := os.ReadFile("testdata/parser/runtime_listener_audit.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			seed, err := ParseInteger(fixture.Seed)
			if err != nil {
				t.Fatal(err)
			}
			rng := NewRNG(seed)
			recorder := NewTraceRecorder(fixture.Verbosity, rng, fixture.Capacity)
			listener := &executionRecorderSourceListener{TraceRecorder: recorder, t: t, events: []executionRecorderEvent{}}
			ctx := WithExecutionListener(context.Background(), listener)
			var value RuntimeValue
			var evaluationError error
			switch fixture.Mode {
			case "expression":
				value, evaluationError = EvaluatePureExpression(ctx, fixture.Input, nil)
			case "module", "project", "transported":
				var evaluator *ModuleEvaluator
				if fixture.Mode != "project" {
					evaluator, err = NewModuleEvaluator(fixture.Module, rng, false)
				} else {
					ids := NewIDGenerator()
					modules, parseError := parseModulesWithIDs(fixture.Source, ids)
					if parseError != nil {
						t.Fatal(parseError)
					}
					evaluator, err = NewProjectEvaluator(ctx, modules, ids, "M", rng, false)
				}
				if err != nil {
					t.Fatal(err)
				}
				value, evaluationError = evaluator.Evaluate(ctx, fixture.Root, nil)
			default:
				t.Fatal("unknown audit mode", fixture.Mode)
			}
			for index := range listener.events {
				listener.events[index].App = runtimeListenerAuditApp(t, listener.events[index].App)
			}
			if fixture.Error != nil {
				var diagnostic Diagnostic
				if !errors.As(evaluationError, &diagnostic) || !reflect.DeepEqual(diagnostic, *fixture.Error) {
					t.Fatalf("diagnostic got %#v (%v), want %#v", diagnostic, evaluationError, *fixture.Error)
				}
			} else {
				if evaluationError != nil {
					t.Fatal(evaluationError)
				}
				output, err := SerializeRuntimeValueContext(ctx, value, nil)
				if err != nil || fixture.Output == nil || !reflect.DeepEqual(output, *fixture.Output) {
					t.Fatalf("output got %#v (%v), want %#v", output, err, fixture.Output)
				}
			}
			if !reflect.DeepEqual(listener.events, fixture.Events) {
				if len(listener.events) != len(fixture.Events) {
					t.Fatalf("got %d events, want %d", len(listener.events), len(fixture.Events))
				}
				for index, actual := range listener.events {
					want := fixture.Events[index]
					if !reflect.DeepEqual(actual, want) {
						t.Fatalf("event %d: got %s %s %s\nwant %s %s %s", index, actual.Kind, actual.App.ID.String(), ExpressionString(actual.App), want.Kind, want.App.ID.String(), ExpressionString(want.App))
					}
				}
			}
			actual := executionRecorderObserve(t, recorder)
			for _, frame := range []*executionRecorderObservedFrame{actual.CurrentFrame, actual.ActiveRoot} {
				if frame != nil {
					runtimeListenerAuditFrame(t, frame)
				}
			}
			if !reflect.DeepEqual(actual, fixture.Final) {
				got, _ := json.Marshal(actual)
				want, _ := json.Marshal(fixture.Final)
				t.Fatalf("tree got %s\nwant %s", got, want)
			}
		})
	}
}

func runtimeListenerAuditApp(t *testing.T, app Expr) Expr {
	t.Helper()
	if app.Kind == "" {
		return app
	}
	// Capture the public IR boundary: private source annotations and absent
	// optional slices do not change an application's source identity or tree.
	data, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}
	var canonical Expr
	if err := json.Unmarshal(data, &canonical); err != nil {
		t.Fatal(err)
	}
	return canonical
}

func runtimeListenerAuditFrame(t *testing.T, frame *executionRecorderObservedFrame) {
	t.Helper()
	frame.App = runtimeListenerAuditApp(t, frame.App)
	for index := range frame.Subframes {
		runtimeListenerAuditFrame(t, &frame.Subframes[index])
	}
}
