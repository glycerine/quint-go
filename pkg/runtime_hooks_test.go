package quint

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamRuntimeDebugOutput(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Debug []struct {
			Source string
			Width  int
			Input  Expr
			Stdout string
			Output *Expr
			Error  *Diagnostic
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Debug {
		t.Run(fixture.Source, func(t *testing.T) {
			var stdout strings.Builder
			ctx := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: &stdout, DebugWidth: fixture.Width})
			got, err := EvaluatePureExpression(ctx, fixture.Input, nil)
			if stdout.String() != fixture.Stdout {
				t.Fatalf("Go %q\nTS %q", stdout.String(), fixture.Stdout)
			}
			if fixture.Error != nil {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != fixture.Error.Code || diagnostic.Message != fixture.Error.Message || diagnostic.Reference == nil || *diagnostic.Reference != *fixture.Error.Reference {
					t.Fatal("diagnostic", err, fixture.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			actual, err := SerializeRuntimeValue(got, nil)
			if err != nil || !reflect.DeepEqual(actual, *fixture.Output) {
				a, _ := json.Marshal(actual)
				b, _ := json.Marshal(*fixture.Output)
				t.Fatalf("Go %s\nTS %s: %v", a, b, err)
			}
		})
	}
}

func TestRuntimeDebugHooksPreserveValuesAndContexts(t *testing.T) {
	t.Parallel()
	label, _ := NewRuntimeValue(Expr{Kind: "str", Str: "label"})
	value := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) { return args[0], nil })
	var first, second strings.Builder
	ctx := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: &first})
	for _, test := range []struct {
		ctx    context.Context
		writer *strings.Builder
	}{
		{ctx, &first},
		{WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: &second, DebugWidth: 80}), &second},
	} {
		got, err := EvalDebugBuiltin(test.ctx, []RuntimeValue{label, value})
		if err != nil || !sameRuntimeObject(got, value) || test.writer.String() != "> label {(_a0) => ...}\n" {
			t.Fatal("debug rebuilt closure or used wrong destination", err, test.writer.String())
		}
	}
	if _, err := EvalDebugBuiltin(WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: io.Discard}), []RuntimeValue{label, value}); err != nil {
		t.Fatal("discard output", err)
	}
	if first.String() != "> label {(_a0) => ...}\n" || second.String() != first.String() {
		t.Fatal("debug context override changed another output")
	}
}

func TestRuntimeDebugValidationAndWriteErrors(t *testing.T) {
	t.Parallel()
	var stdout strings.Builder
	ctx := WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: &stdout})
	label, _ := NewRuntimeValue(Expr{Kind: "str", Str: "label"})
	truth, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	for _, args := range [][]RuntimeValue{nil, {label}, {label, truth, truth}, {{}, truth}, {label, {}}, {truth, truth}} {
		if _, err := EvalDebugBuiltin(ctx, args); err == nil || stdout.Len() != 0 {
			t.Fatal("invalid debug call wrote output", err, stdout.String())
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := EvalDebugBuiltin(canceled, []RuntimeValue{label, truth}); !errors.Is(err, context.Canceled) || stdout.Len() != 0 {
		t.Fatal("canceled debug wrote output", err)
	}
	closed := WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: runtimeDebugErrorWriter{}})
	if _, err := EvalDebugBuiltin(closed, []RuntimeValue{label, truth}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("write failure hidden", err)
	}
	expression, err := ParseExpression("Set(0,1,2,3,4,5,6,7,8)")
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	objects := &RuntimeObjectHashState{}
	if _, err = EvalDebugBuiltin(ctx, []RuntimeValue{truth, set}, objects); err == nil || objects.Counter() != 18 || stdout.Len() != 0 {
		t.Fatal("label conversion must follow value serialization", err, objects.Counter())
	}
}

type runtimeDebugErrorWriter struct{}

func (runtimeDebugErrorWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
