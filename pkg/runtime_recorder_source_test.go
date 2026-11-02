package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

// These are captured from the actual TypeScript evaluator, rather than events
// supplied directly to a recorder. The fixture retains source IR IDs so caller
// labels, partial argument evaluations, and diagnostic references stay exact.
func TestUpstreamExecutionRecorderSource(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source, Mode, Root, Seed  string
		Module                          Module
		Input                           Expr
		Verbosity, Capacity, MaxSamples int
		Options                         SimulationOptions
		Output                          *Expr
		Error                           *Diagnostic
		Events                          []executionRecorderEvent
		Final                           executionRecorderSnapshot
	}
	data, err := os.ReadFile("testdata/parser/runtime_recorder_source.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
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
			var result RuntimeValue
			var evaluationError error
			if fixture.Mode == "expression" {
				result, evaluationError = EvaluatePureExpression(ctx, fixture.Input, nil)
			} else {
				evaluator, err := NewModuleEvaluator(fixture.Module, rng, false)
				if err != nil {
					t.Fatal(err)
				}
				switch fixture.Mode {
				case "module":
					result, evaluationError = evaluator.Evaluate(ctx, fixture.Root, nil)
				case "test":
					_, evaluationError = evaluator.Test(ctx, fixture.Root, fixture.MaxSamples)
				case "simulate":
					_, evaluationError = evaluator.Simulate(ctx, fixture.Options)
				default:
					t.Fatal("unknown fixture mode", fixture.Mode)
				}
			}
			if fixture.Mode == "expression" || fixture.Mode == "module" {
				if fixture.Error != nil {
					var diagnostic Diagnostic
					if !errors.As(evaluationError, &diagnostic) || diagnostic.Code != fixture.Error.Code || diagnostic.Message != fixture.Error.Message || !reflect.DeepEqual(diagnostic.Reference, fixture.Error.Reference) {
						t.Fatalf("got %v; want %#v", evaluationError, fixture.Error)
					}
				} else {
					if evaluationError != nil {
						t.Fatal(evaluationError)
					}
					output, err := SerializeRuntimeValueContext(ctx, result, nil)
					if err != nil || fixture.Output == nil || !reflect.DeepEqual(output, *fixture.Output) {
						t.Fatalf("output got %#v (%v), want %#v", output, err, fixture.Output)
					}
				}
			} else if evaluationError != nil {
				t.Fatal(evaluationError)
			}
			if !reflect.DeepEqual(listener.events, fixture.Events) {
				got, _ := json.Marshal(listener.events)
				want, _ := json.Marshal(fixture.Events)
				t.Fatalf("events got %s\nwant %s", got, want)
			}
			if got := executionRecorderObserve(t, recorder); !reflect.DeepEqual(got, fixture.Final) {
				actual, _ := json.Marshal(got)
				want, _ := json.Marshal(fixture.Final)
				t.Fatalf("frames got %s\nwant %s", actual, want)
			}
		})
	}
}

type executionRecorderSourceListener struct {
	*TraceRecorder
	t      *testing.T
	events []executionRecorderEvent
}

func (l *executionRecorderSourceListener) expressions(values []RuntimeValue) []Expr {
	l.t.Helper()
	result := make([]Expr, len(values))
	for i, value := range values {
		var err error
		result[i], err = SerializeRuntimeValueContext(context.Background(), value, nil)
		if err != nil {
			l.t.Fatal(err)
		}
	}
	return result
}
func (l *executionRecorderSourceListener) outcome(event *executionRecorderEvent, result RuntimeValue, err error) {
	l.t.Helper()
	if err != nil {
		var diagnostic Diagnostic
		if !errors.As(err, &diagnostic) {
			l.t.Fatal(err)
		}
		event.Error = &executionRecorderObservedError{Code: diagnostic.Code, Message: diagnostic.Message}
	} else {
		expression := l.expressions([]RuntimeValue{result})[0]
		event.Result = &expression
	}
}
func (l *executionRecorderSourceListener) OnUserOperatorCall(app Expr) {
	l.events = append(l.events, executionRecorderEvent{Kind: "call", App: copyRuntimeExpression(app)})
	l.TraceRecorder.OnUserOperatorCall(app)
}
func (l *executionRecorderSourceListener) OnUserOperatorReturn(app Expr, args []RuntimeValue, result RuntimeValue, err error) {
	event := executionRecorderEvent{Kind: "return", App: copyRuntimeExpression(app), Args: l.expressions(args)}
	l.outcome(&event, result, err)
	l.events = append(l.events, event)
	l.TraceRecorder.OnUserOperatorReturn(app, args, result, err)
}
func (l *executionRecorderSourceListener) OnAnyOptionCall(app Expr, position int) {
	l.events = append(l.events, executionRecorderEvent{Kind: "anyCall", App: copyRuntimeExpression(app), Position: position})
	l.TraceRecorder.OnAnyOptionCall(app, position)
}
func (l *executionRecorderSourceListener) OnAnyOptionReturn(app Expr, position int) {
	l.events = append(l.events, executionRecorderEvent{Kind: "anyReturnOption", App: copyRuntimeExpression(app), Position: position})
	l.TraceRecorder.OnAnyOptionReturn(app, position)
}
func (l *executionRecorderSourceListener) OnAnyReturn(noptions, choice int) {
	l.events = append(l.events, executionRecorderEvent{Kind: "anyReturn", Noptions: noptions, Choice: choice})
	l.TraceRecorder.OnAnyReturn(noptions, choice)
}
func (l *executionRecorderSourceListener) OnNextState(oldState, newState RuntimeValue) {
	values := l.expressions([]RuntimeValue{oldState, newState})
	l.events = append(l.events, executionRecorderEvent{Kind: "nextState", OldState: values[0], NewState: values[1]})
	l.TraceRecorder.OnNextState(oldState, newState)
}
func (l *executionRecorderSourceListener) OnRunCall() {
	l.events = append(l.events, executionRecorderEvent{Kind: "runCall"})
	l.TraceRecorder.OnRunCall()
}
func (l *executionRecorderSourceListener) OnRunReturn(result RuntimeValue, err error, trace []RuntimeValue) {
	event := executionRecorderEvent{Kind: "runReturn", Trace: l.expressions(trace)}
	l.outcome(&event, result, err)
	l.events = append(l.events, event)
	l.TraceRecorder.OnRunReturn(result, err, trace)
}

func TestExecutionRecorderSavedClosureContexts(t *testing.T) {
	t.Parallel()
	modules, err := ParseModules("module M { pure def inc(x)=x+1 pure val saved=y=>inc(y) }")
	if err != nil {
		t.Fatal(err)
	}
	evaluator, err := NewModuleEvaluator(modules[0], nil, false)
	if err != nil {
		t.Fatal(err)
	}
	first, second := NewTraceRecorder(3, evaluator.RNG), NewTraceRecorder(3, evaluator.RNG)
	saved, err := evaluator.Evaluate(WithExecutionListener(context.Background(), first), "saved", nil)
	if err != nil {
		t.Fatal(err)
	}
	one, _ := ParseInteger("1")
	argument, _ := NewRuntimeValue(Expr{Kind: "int", Int: one})
	for _, ctx := range []context.Context{
		WithExecutionListener(context.Background(), first),
		WithExecutionListener(context.Background(), second),
		context.Background(),
	} {
		result, err := saved.Apply(ctx, []RuntimeValue{argument})
		if err != nil || result.String() != "2" {
			t.Fatal(result.String(), err)
		}
	}
	for i, recorder := range []*TraceRecorder{first, second} {
		root := recorder.CurrentFrame()
		if len(root.Subframes) != 1 || root.Subframes[0].App.Opcode != "inc" || len(root.Subframes[0].Args) != 1 || root.Subframes[0].Result.String() != "2" {
			t.Fatalf("recorder %d captured wrong closure context: %#v", i, root)
		}
	}
}
