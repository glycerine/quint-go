package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestExecutionFormattingContextUpstreamDomains(t *testing.T) {
	t.Parallel()
	// These saved evaluator vectors contain the actual upstream materialized
	// expressions and thrown exceptions. Frame layout has its own exact oracle.
	var fixtures struct {
		Cases []struct {
			Name    string
			Input   Expr
			Output  Expr
			Thrown  *struct{ Name, Message string }
			Counter uint32
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_symbolic_expression.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{
		"integer_display": true, "natural_display": true,
		"interval_numeric_display": true, "interval_negative_display": true, "interval_empty_display": true,
		"powerset_display": true, "product_display": true, "mapset_display": true,
		"nested_domain_display": true, "infinite_nested_display": true,
		"product_enumeration_error": true, "powerset_infinite_enumeration_error": true,
		"mapset_empty_domain_infinite_display_error": true,
	}
	for _, fixture := range fixtures.Cases {
		if !selected[fixture.Name] {
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			objects := &RuntimeObjectHashState{}
			value, err := EvaluatePureExpressionWithRuntimeState(ctx, fixture.Input, nil, objects)
			if err != nil {
				t.Fatal("evaluation before display", err)
			}
			frame := ExecutionFrame{App: Expr{Kind: "app", Opcode: "show"}, Result: value}
			text, err := FormatExecutionFrameContext(ctx, frame, 10000, objects)
			if fixture.Thrown != nil {
				var thrown RuntimeDomainException
				if !errors.As(err, &thrown) || thrown.Name != fixture.Thrown.Name || thrown.Message != fixture.Thrown.Message || text != "" {
					t.Fatalf("got %q (%v); want %v", text, err, fixture.Thrown)
				}
			} else {
				formatted, formatErr := FormatValueExpression(fixture.Output, 10000, 8)
				want := "show => " + formatted + "\n"
				if formatErr != nil || err != nil || text != want {
					t.Fatalf("got %q (%v); want %q (%v)", text, err, want, formatErr)
				}
			}
			if counter := objects.Counter(); counter != fixture.Counter {
				t.Fatalf("object counter: got %d, want %d", counter, fixture.Counter)
			}
		})
	}
}

func TestExecutionFormattingContextArgumentsAndCancellation(t *testing.T) {
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
	frame := ExecutionFrame{App: Expr{Kind: "app", Opcode: "call"}, Args: []RuntimeValue{value}, Err: errors.New("captured failure")}
	text, err := FormatExecutionFrameContext(ctx, frame, 80, nil)
	if err != nil || text != "call(Set(1, 2, 3)) => none\n" {
		t.Fatalf("domain argument: got %q (%v)", text, err)
	}
	if _, err := FormatExecutionFrame(frame, 80); err == nil {
		t.Fatal("legacy formatter accepted a domain without a context")
	}
	if _, err := FormatExecutionFrameContext(ctx, frame, -1); err == nil {
		t.Fatal("negative width accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := FormatExecutionFrameContext(canceled, ExecutionFrame{Err: errors.New("failure without values")}, 80); !errors.Is(err, context.Canceled) {
		t.Fatal("empty failed frame ignored cancellation", err)
	}
	expression, err = ParseExpression("0.to(1000000000000000000000000000000)")
	if err != nil {
		t.Fatal(err)
	}
	value, err = EvaluatePureExpression(ctx, expression, nil)
	if err != nil {
		t.Fatal(err)
	}
	limited := &executionFormattingCancellation{Context: ctx, remaining: 12}
	frame.Args = []RuntimeValue{value}
	if text, err := FormatExecutionFrameContext(limited, frame, 80); text != "" || !errors.Is(err, context.Canceled) || limited.remaining != 0 {
		t.Fatalf("huge domain was not canceled: text=%q remaining=%d error=%v", text, limited.remaining, err)
	}
	truth, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	frame.Args = nil
	frame.Err, frame.Result = nil, truth
	frame.Subframes = []ExecutionFrame{{App: Expr{Kind: "app", Opcode: "child"}, Result: truth}}
	// The first frame can be formatted successfully before a later frame notices
	// cancellation. Return no partial tree when that occurs.
	limited = &executionFormattingCancellation{Context: ctx, remaining: 5}
	if text, err := FormatExecutionFrameContext(limited, frame, 80); text != "" || !errors.Is(err, context.Canceled) {
		t.Fatalf("partial tree escaped cancellation: %q (%v)", text, err)
	}
}

type executionFormattingCancellation struct {
	context.Context
	remaining int
}

func (c *executionFormattingCancellation) Err() error {
	if c.remaining > 0 {
		c.remaining--
	}
	if c.remaining == 0 {
		return context.Canceled
	}
	return nil
}
