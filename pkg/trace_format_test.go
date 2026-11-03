package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/glycerine/quint-go/pkg/terminal"
)

type traceFormatFixtureFrame struct {
	App       Expr
	Args      []Expr
	Result    *Expr
	Error     *Diagnostic
	Subframes []traceFormatFixtureFrame
}

func traceFormatFixtureFrames(t *testing.T, input []traceFormatFixtureFrame) []ExecutionFrame {
	t.Helper()
	frames := make([]ExecutionFrame, len(input))
	for i, frame := range input {
		frames[i].App = frame.App
		for _, argument := range frame.Args {
			value, err := NewRuntimeValue(argument)
			if err != nil {
				t.Fatal(err)
			}
			frames[i].Args = append(frames[i].Args, value)
		}
		if frame.Error != nil {
			frames[i].Err = *frame.Error
		} else if frame.Result != nil {
			value, err := NewRuntimeValue(*frame.Result)
			if err != nil {
				t.Fatal(err)
			}
			frames[i].Result = value
		}
		frames[i].Subframes = traceFormatFixtureFrames(t, frame.Subframes)
	}
	return frames
}

func TestUpstreamTraceDisplay(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Traces []struct {
			Name         string
			States       []Expr
			Diagnostics  [][]DebugMessage
			Pending      []DebugMessage
			Frames       []traceFormatFixtureFrame
			Hide         []string
			Width, Level int
			Output       string
		}
	}
	data, err := os.ReadFile("testdata/parser/trace_display.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Traces {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			ctx := terminal.WithOptions(context.Background(), terminal.Options{ColorLevel: &fixture.Level})
			options := TraceFormatOptions{Frames: traceFormatFixtureFrames(t, fixture.Frames), Diagnostics: fixture.Diagnostics, PendingDiagnostics: fixture.Pending, HideVariables: fixture.Hide}
			before, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			got, err := FormatTraceContext(ctx, fixture.States, fixture.Width, options)
			if err != nil || got != fixture.Output {
				t.Fatalf("got %q err=%v\nwant %q", got, err, fixture.Output)
			}
			if fixture.Level == 0 {
				plain, err := FormatTrace(fixture.States, fixture.Width, options)
				if err != nil || plain != fixture.Output {
					t.Fatal(plain, err)
				}
			}
			after, err := json.Marshal(fixture)
			if err != nil || string(after) != string(before) {
				t.Fatal("trace formatting changed caller-owned annotations or expressions", err)
			}
		})
	}
}

func TestTraceFormattingValidationAndCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	state := Expr{Kind: "app", Opcode: "Rec", Args: []Expr{}}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := FormatTraceContext(canceled, nil, 80); got != "" || !errors.Is(err, context.Canceled) {
		t.Fatal(got, err)
	}
	for _, test := range []struct {
		states  []Expr
		width   int
		options TraceFormatOptions
	}{
		{nil, -1, TraceFormatOptions{}},
		{[]Expr{{Kind: "bool"}}, 80, TraceFormatOptions{}},
		{[]Expr{{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}}}}, 80, TraceFormatOptions{}},
		{[]Expr{{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}, {Kind: "app", Opcode: "variant"}}}}, 80, TraceFormatOptions{}},
		{[]Expr{state}, 80, TraceFormatOptions{Frames: []ExecutionFrame{{}}}},
		{[]Expr{state}, 80, TraceFormatOptions{Diagnostics: [][]DebugMessage{{{Value: Expr{Kind: "app", Opcode: "variant"}}}}}},
		{[]Expr{state}, 80, TraceFormatOptions{PendingDiagnostics: []DebugMessage{{Value: Expr{Kind: "app", Opcode: "variant"}}}}},
	} {
		if got, err := FormatTraceContext(ctx, test.states, test.width, test.options); err == nil || got != "" {
			t.Fatal("invalid trace produced partial output", got, err)
		}
	}
	if text, err := FormatTrace(nil, 80, TraceFormatOptions{Frames: []ExecutionFrame{{}}}); err != nil || text != "" {
		t.Fatal("extra frames should be ignored", text, err)
	}
	bounded := &traceFormatCancellation{Context: ctx, remaining: 3}
	if text, err := FormatTraceContext(bounded, []Expr{state, state, state}, 80); text != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled trace returned partial output", text, err)
	}
	bounded = &traceFormatCancellation{Context: ctx, remaining: 4}
	if text, err := FormatTraceContext(bounded, []Expr{state, state}, 80); text != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation between states returned partial output", text, err)
	}
	bounded = &traceFormatCancellation{Context: ctx, remaining: 3}
	if text, err := FormatTraceContext(bounded, nil, 80, TraceFormatOptions{PendingDiagnostics: []DebugMessage{{Value: Expr{Kind: "bool"}}, {Value: Expr{Kind: "bool"}}}}); text != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("pending diagnostics ignored cancellation", text, err)
	}
	bounded = &traceFormatCancellation{Context: ctx, remaining: 2}
	if text, err := FormatTraceContext(bounded, nil, 80); text != "" || !errors.Is(err, context.Canceled) {
		t.Fatal("empty trace lost cancellation before returning", text, err)
	}
}

func TestTraceFormattingCapturedDomainsAndSharedHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	expression, err := ParseExpression("1.to(3)")
	if err != nil {
		t.Fatal(err)
	}
	value, err := EvaluatePureExpression(ctx, expression, nil)
	if err != nil {
		t.Fatal(err)
	}
	state := Expr{Kind: "app", Opcode: "Rec", Args: []Expr{}}
	frame := ExecutionFrame{App: Expr{Kind: "app", Opcode: "identity"}, Args: []RuntimeValue{value}, Result: value}
	got, err := FormatTraceContext(ctx, []Expr{state}, 80, TraceFormatOptions{Frames: []ExecutionFrame{frame}})
	if err != nil || got != "[Frame 0]\nidentity(Set(1, 2, 3)) => Set(1, 2, 3)\n\n[State 0] {  }\n\n" {
		t.Fatal(got, err)
	}
	expression, err = ParseExpression("Set(1,2,3,4,5,6,7,8,9)")
	if err != nil {
		t.Fatal(err)
	}
	value, err = NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	before := value.Expression()
	objects := &RuntimeObjectHashState{}
	frame.Args, frame.Result = []RuntimeValue{value}, value
	if _, err := FormatTraceContext(ctx, []Expr{state}, 80, TraceFormatOptions{Frames: []ExecutionFrame{frame}, ObjectHashState: objects}); err != nil || objects.Counter() == 0 || !reflect.DeepEqual(before, value.Expression()) {
		t.Fatal("trace discarded display history or mutated its runtime value", objects.Counter(), err)
	}
}

type traceFormatCancellation struct {
	context.Context
	remaining int
}

func (c *traceFormatCancellation) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}
