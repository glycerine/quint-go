package quint

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type executionFixtureFrame struct {
	App       Expr
	Args      []Expr
	Result    *Expr
	Error     bool
	Subframes []executionFixtureFrame
}

func TestUpstreamExecutionFormatting(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		StateCases []struct {
			Name         string
			Input        Expr
			Index, Width int
			Output       string
		}
		FrameCases []struct {
			Name   string
			Input  executionFixtureFrame
			Width  int
			Output string
		}
	}
	data, err := os.ReadFile("testdata/parser/execution_format.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.StateCases {
		got, err := FormatTraceStateExpression(fixture.Index, fixture.Input, fixture.Width)
		if err != nil || got != fixture.Output {
			t.Fatalf("%s: got %q want %q: %v", fixture.Name, got, fixture.Output, err)
		}
	}
	var convert func(executionFixtureFrame) ExecutionFrame
	convert = func(input executionFixtureFrame) ExecutionFrame {
		output := ExecutionFrame{App: input.App}
		for _, arg := range input.Args {
			value, err := NewRuntimeValue(arg)
			if err != nil {
				t.Fatal(err)
			}
			output.Args = append(output.Args, value)
		}
		if input.Error {
			output.Err = errors.New("captured failure")
		} else {
			value, err := NewRuntimeValue(*input.Result)
			if err != nil {
				t.Fatal(err)
			}
			output.Result = value
		}
		for _, child := range input.Subframes {
			output.Subframes = append(output.Subframes, convert(child))
		}
		return output
	}
	for _, fixture := range fixtures.FrameCases {
		got, err := FormatExecutionFrame(convert(fixture.Input), fixture.Width)
		if err != nil || got != fixture.Output {
			t.Fatalf("%s: got %q want %q: %v", fixture.Name, got, fixture.Output, err)
		}
	}
}

func TestExecutionFormattingValidation(t *testing.T) {
	t.Parallel()
	truth, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	valid := ExecutionFrame{App: Expr{Kind: "app", Opcode: "call"}, Result: truth}
	if _, err := FormatExecutionFrame(valid, -1); err == nil {
		t.Fatal("negative width accepted")
	}
	if _, err := FormatExecutionFrame(ExecutionFrame{Args: []RuntimeValue{{}}, Result: truth}, 80); err == nil {
		t.Fatal("invalid argument accepted")
	}
	if _, err := FormatExecutionFrame(ExecutionFrame{}, 80); err == nil {
		t.Fatal("invalid successful result accepted")
	}
	valid.Subframes = []ExecutionFrame{{}}
	if _, err := FormatExecutionFrame(valid, 80); err == nil {
		t.Fatal("invalid child frame accepted")
	}
	for _, test := range []struct {
		index int
		value Expr
		width int
	}{
		{-1, Expr{Kind: "app", Opcode: "Rec"}, 80}, {0, Expr{Kind: "app", Opcode: "Rec"}, -1},
		{0, Expr{Kind: "bool"}, 80}, {0, Expr{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}}}, 80},
	} {
		if _, err := FormatTraceStateExpression(test.index, test.value, test.width); err == nil {
			t.Fatal("invalid state format accepted", test)
		}
	}
}
