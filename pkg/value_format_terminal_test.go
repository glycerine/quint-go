package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/glycerine/quint-go/pkg/terminal"
)

func TestUpstreamTerminalValueFormatting(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		ExpressionCases []struct {
			Name               string
			Level              int
			Input              Expr
			Width, FirstColumn int
			Output             string
		}
		StateCases []struct {
			Name         string
			Level        int
			Input        Expr
			Index, Width int
			Output       string
		}
		FrameCases []struct {
			Name   string
			Level  int
			Input  executionFixtureFrame
			Width  int
			Output string
		}
	}
	data, err := os.ReadFile("testdata/parser/value_terminal.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	contextFor := func(level int) context.Context {
		return terminal.WithOptions(context.Background(), terminal.Options{ColorLevel: &level})
	}
	for _, fixture := range fixtures.ExpressionCases {
		got, err := FormatValueExpressionContext(contextFor(fixture.Level), fixture.Input, fixture.Width, fixture.FirstColumn)
		if err != nil || got != fixture.Output {
			t.Fatalf("%s got=%q want=%q err=%v", fixture.Name, got, fixture.Output, err)
		}
	}
	for _, fixture := range fixtures.StateCases {
		got, err := FormatTraceStateExpressionContext(contextFor(fixture.Level), fixture.Index, fixture.Input, fixture.Width)
		if err != nil || got != fixture.Output {
			t.Fatalf("%s got=%q want=%q err=%v", fixture.Name, got, fixture.Output, err)
		}
	}
	var frame func(executionFixtureFrame) ExecutionFrame
	frame = func(input executionFixtureFrame) ExecutionFrame {
		result := ExecutionFrame{App: input.App}
		for _, expr := range input.Args {
			value, err := NewRuntimeValue(expr)
			if err != nil {
				t.Fatal(err)
			}
			result.Args = append(result.Args, value)
		}
		if input.Error {
			result.Err = errors.New("captured error")
		} else {
			value, err := NewRuntimeValue(*input.Result)
			if err != nil {
				t.Fatal(err)
			}
			result.Result = value
		}
		for _, child := range input.Subframes {
			result.Subframes = append(result.Subframes, frame(child))
		}
		return result
	}
	for _, fixture := range fixtures.FrameCases {
		got, err := FormatExecutionFrameContext(contextFor(fixture.Level), frame(fixture.Input), fixture.Width)
		if err != nil || got != fixture.Output {
			t.Fatalf("%s got=%q want=%q err=%v", fixture.Name, got, fixture.Output, err)
		}
	}
}

func TestTerminalValueFormattingValidation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := FormatValueExpressionContext(ctx, Expr{}, 80, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := FormatTraceStateExpressionContext(ctx, 0, Expr{}, 80); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, test := range []struct{ width, column int }{{-1, 0}, {80, -1}} {
		if _, err := FormatValueExpressionContext(context.Background(), Expr{}, test.width, test.column); err == nil {
			t.Fatal("invalid layout accepted")
		}
	}
	if _, err := FormatValueExpressionContext(context.Background(), Expr{Kind: "app", Opcode: "variant"}, 80, 0); err == nil {
		t.Fatal("invalid value accepted")
	}
}
