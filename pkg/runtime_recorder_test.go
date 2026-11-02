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

type executionRecorderObservedFrame struct {
	App       Expr                             `json:"app"`
	Args      []Expr                           `json:"args"`
	Result    *Expr                            `json:"result"`
	Error     *executionRecorderObservedError  `json:"error"`
	Subframes []executionRecorderObservedFrame `json:"subframes"`
}

type executionRecorderObservedError struct {
	Code, Message string
}

type executionRecorderObservedTrace struct {
	Frame executionRecorderObservedFrame `json:"frame"`
	Seed  string                         `json:"seed"`
}

type executionRecorderSnapshot struct {
	CurrentFrame *executionRecorderObservedFrame  `json:"currentFrame"`
	ActiveRoot   *executionRecorderObservedFrame  `json:"activeRoot"`
	BestTraces   []executionRecorderObservedTrace `json:"bestTraces"`
	RNG          string                           `json:"rng"`
}

type executionRecorderEvent struct {
	Kind               string
	App                Expr
	Args               []Expr
	Result             *Expr
	Error              *executionRecorderObservedError
	Position           int
	Noptions, Choice   int
	OldState, NewState Expr
	Trace              []Expr
	Bound              string
	Verbosity          int
}

func TestUpstreamExecutionRecorder(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name                string
		Verbosity, Capacity int
		Seed                string
		Initial             executionRecorderSnapshot
		Steps               []struct {
			Event    executionRecorderEvent
			Snapshot executionRecorderSnapshot
		}
	}
	data, err := os.ReadFile("testdata/parser/execution_recorder.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			seed, err := ParseInteger(fixture.Seed)
			if err != nil {
				t.Fatal(err)
			}
			rng := NewRNG(seed)
			recorder := NewTraceRecorder(fixture.Verbosity, rng, fixture.Capacity)
			check := func(t *testing.T, want executionRecorderSnapshot) {
				t.Helper()
				got := executionRecorderObserve(t, recorder)
				if !reflect.DeepEqual(got, want) {
					actual, _ := json.Marshal(got)
					expected, _ := json.Marshal(want)
					t.Fatalf("got %s\nwant %s", actual, expected)
				}
			}
			check(t, fixture.Initial)
			for i, step := range fixture.Steps {
				t.Run(step.Event.Kind+"_"+strconv.Itoa(i), func(t *testing.T) {
					executionRecorderApply(t, recorder, step.Event)
					check(t, step.Snapshot)
				})
			}
		})
	}
}

func executionRecorderValue(t *testing.T, expression Expr) RuntimeValue {
	t.Helper()
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func executionRecorderApply(t *testing.T, recorder *TraceRecorder, event executionRecorderEvent) {
	t.Helper()
	values := func(expressions []Expr) []RuntimeValue {
		result := make([]RuntimeValue, len(expressions))
		for i, expression := range expressions {
			result[i] = executionRecorderValue(t, expression)
		}
		return result
	}
	var value RuntimeValue
	var failure error
	if event.Result != nil {
		value = executionRecorderValue(t, *event.Result)
	}
	if event.Error != nil {
		failure = Diagnostic{Code: event.Error.Code, Message: event.Error.Message}
	}
	switch event.Kind {
	case "call":
		recorder.OnUserOperatorCall(event.App)
	case "return":
		recorder.OnUserOperatorReturn(event.App, values(event.Args), value, failure)
	case "anyCall":
		recorder.OnAnyOptionCall(event.App, event.Position)
	case "anyReturnOption":
		recorder.OnAnyOptionReturn(event.App, event.Position)
	case "anyReturn":
		recorder.OnAnyReturn(event.Noptions, event.Choice)
	case "nextState":
		recorder.OnNextState(executionRecorderValue(t, event.OldState), executionRecorderValue(t, event.NewState))
	case "runCall":
		recorder.OnRunCall()
	case "runReturn":
		recorder.OnRunReturn(value, failure, values(event.Trace))
	case "clear":
		recorder.Clear()
	case "draw":
		bound, err := ParseInteger(event.Bound)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := recorder.RNG.Next(bound); err != nil {
			t.Fatal(err)
		}
	case "setVerbosity":
		recorder.VerbosityLevel = event.Verbosity
	default:
		t.Fatal("unknown recorder event", event.Kind)
	}
}

func executionRecorderObserve(t *testing.T, recorder *TraceRecorder) executionRecorderSnapshot {
	t.Helper()
	var observe func(ExecutionFrame) executionRecorderObservedFrame
	observe = func(frame ExecutionFrame) executionRecorderObservedFrame {
		result := executionRecorderObservedFrame{App: frame.App, Args: make([]Expr, len(frame.Args)), Subframes: make([]executionRecorderObservedFrame, len(frame.Subframes))}
		for i, value := range frame.Args {
			var err error
			result.Args[i], err = SerializeRuntimeValueContext(context.Background(), value, nil)
			if err != nil {
				t.Fatal(err)
			}
		}
		if frame.Err != nil {
			var diagnostic Diagnostic
			if !errors.As(frame.Err, &diagnostic) {
				t.Fatal("expected diagnostic", frame.Err)
			}
			result.Error = &executionRecorderObservedError{Code: diagnostic.Code, Message: diagnostic.Message}
		} else {
			expression, err := SerializeRuntimeValueContext(context.Background(), frame.Result, nil)
			if err != nil {
				t.Fatal(err)
			}
			result.Result = &expression
		}
		for i, child := range frame.Subframes {
			result.Subframes[i] = observe(child)
		}
		return result
	}
	current := observe(recorder.CurrentFrame())
	result := executionRecorderSnapshot{CurrentFrame: &current, BestTraces: []executionRecorderObservedTrace{}, RNG: recorder.RNG.State().String()}
	if active := recorder.ActiveFrame(); active.App.Kind != "" {
		root := observe(active)
		result.ActiveRoot = &root
	}
	for _, trace := range recorder.BestTraces() {
		result.BestTraces = append(result.BestTraces, executionRecorderObservedTrace{Frame: observe(trace.Frame), Seed: trace.Seed.String()})
	}
	return result
}

func TestExecutionRecorderOwnershipAndContext(t *testing.T) {
	t.Parallel()
	recorder := NewTraceRecorder(3, nil)
	ctx := WithExecutionListener(context.Background(), recorder)
	if ExecutionListenerFromContext(ctx) != recorder || ExecutionListenerFromContext(context.Background()) != nil || ExecutionListenerFromContext(WithExecutionListener(ctx, nil)) != nil {
		t.Fatal("listener context propagation")
	}
	app := Expr{Kind: "app", Opcode: "call", Args: []Expr{{Kind: "bool", Bool: true}}}
	truth := executionRecorderValue(t, Expr{Kind: "bool", Bool: true})
	falsity := executionRecorderValue(t, Expr{Kind: "bool"})
	recorder.OnUserOperatorCall(app)
	app.Args[0].Bool = false
	args := []RuntimeValue{truth}
	recorder.OnUserOperatorReturn(app, args, truth, nil)
	args[0] = falsity
	states := []RuntimeValue{truth}
	recorder.OnRunReturn(truth, nil, states)
	states[0] = falsity
	before := executionRecorderObserve(t, recorder)
	if !before.CurrentFrame.Subframes[0].App.Args[0].Bool || !before.CurrentFrame.Subframes[0].Args[0].Bool || !before.CurrentFrame.Args[0].Bool {
		t.Fatal("recorder retained mutable caller slices")
	}
	frame := recorder.CurrentFrame()
	frame.App.Opcode = "changed"
	frame.Args[0] = falsity
	frame.Subframes[0].App.Args[0].Bool = false
	frame.Subframes[0].Args[0] = falsity
	traces := recorder.BestTraces()
	traces[0].Frame.Subframes[0].App.Opcode = "changed"
	traces[0].Frame.Args[0] = falsity
	active := recorder.ActiveFrame()
	active.Subframes[0].App.Opcode = "changed"
	if after := executionRecorderObserve(t, recorder); !reflect.DeepEqual(before, after) {
		t.Fatal("snapshot mutation altered recorder")
	}
	recorder.OnRunCall()
	recorder.OnRunReturn(falsity, nil, nil)
	if len(recorder.BestTraces()) != 1 || recorder.BestTraces()[0].Frame.Result.expression.Bool {
		t.Fatal("default retention capacity or failing-run priority")
	}
}

func TestExecutionRecorderNilAndEmptyStack(t *testing.T) {
	t.Parallel()
	var recorder *TraceRecorder
	recorder.Clear()
	recorder.OnUserOperatorCall(Expr{})
	recorder.OnUserOperatorReturn(Expr{}, nil, RuntimeValue{}, nil)
	recorder.OnAnyOptionCall(Expr{}, 0)
	recorder.OnAnyOptionReturn(Expr{}, 0)
	recorder.OnAnyReturn(0, -1)
	recorder.OnNextState(RuntimeValue{}, RuntimeValue{})
	recorder.OnRunCall()
	recorder.OnRunReturn(RuntimeValue{}, nil, nil)
	if recorder.CurrentFrame().App.Kind != "" || recorder.ActiveFrame().App.Kind != "" || recorder.BestTraces() != nil {
		t.Fatal("nil recorder returned a frame")
	}
	var zero TraceRecorder
	if zero.CurrentFrame().App.Kind != "" || zero.ActiveFrame().App.Kind != "" {
		t.Fatal("uninitialized recorder returned a frame")
	}
	recorder = NewTraceRecorder(3, nil, -1)
	recorder.OnUserOperatorReturn(Expr{}, nil, RuntimeValue{}, nil)
	recorder.OnUserOperatorReturn(Expr{}, nil, RuntimeValue{}, nil)
	recorder.OnAnyOptionReturn(Expr{}, 0)
	recorder.OnAnyReturn(0, -1)
	recorder.OnNextState(RuntimeValue{}, RuntimeValue{})
	recorder.OnRunReturn(RuntimeValue{}, nil, nil)
	if recorder.ActiveFrame().App.Kind != "" {
		t.Fatal("empty stack unexpectedly created a frame")
	}
	recorder.OnRunCall()
	truth := executionRecorderValue(t, Expr{Kind: "bool", Bool: true})
	recorder.RNG = nil
	recorder.OnRunCall()
	recorder.OnRunReturn(truth, nil, nil)
	if len(recorder.BestTraces()) != 0 {
		t.Fatal("negative capacity retained a trace")
	}
}

func TestExecutionRecorderRetainsExecutableAndCompactValues(t *testing.T) {
	t.Parallel()
	first, _ := ParseInteger("1")
	last, _ := ParseInteger("3")
	domain, err := RuntimeValueFromDomain(NewRuntimeInterval(first, last))
	if err != nil {
		t.Fatal(err)
	}
	truth := executionRecorderValue(t, Expr{Kind: "bool", Bool: true})
	closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return truth, nil })
	recorder := NewTraceRecorder(3, nil)
	app := Expr{Kind: "app", Opcode: "retained"}
	recorder.OnUserOperatorCall(app)
	recorder.OnUserOperatorReturn(app, []RuntimeValue{closure, domain}, domain, nil)
	recorder.OnRunReturn(truth, nil, []RuntimeValue{domain})
	frame := recorder.BestTraces()[0].Frame
	call := frame.Subframes[0]
	if frame.Args[0].domain != domain.domain || call.Result.domain != domain.domain || call.Args[1].domain != domain.domain || call.Args[0].closure != closure.closure {
		t.Fatal("recording materialized a domain or replaced an executable closure")
	}
	if result, err := call.Args[0].Apply(context.Background(), nil); err != nil || !result.Equal(truth) {
		t.Fatal("captured closure stopped being executable", err)
	}
	if _, err := FormatExecutionFrameContext(context.Background(), call, 80); err != nil {
		t.Fatal("captured compact values cannot be displayed", err)
	}
}
