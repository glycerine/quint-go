package quint

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"testing"
)

func TestREPLFormatValueContextUpstreamDomains(t *testing.T) {
	t.Parallel()
	// Reuse saved actual source-level evaluator output and exception vectors;
	// the formatter must materialize domains in the same traversal and history.
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
			session := NewREPLSession()
			ctx := context.Background()
			value, err := EvaluatePureExpressionWithRuntimeState(ctx, fixture.Input, nil, session.runtime.ObjectHashState())
			if err != nil {
				t.Fatal("evaluation before display", err)
			}
			text, err := session.FormatValueContext(ctx, value, 80)
			if fixture.Thrown != nil {
				var thrown RuntimeDomainException
				if !errors.As(err, &thrown) || thrown.Name != fixture.Thrown.Name || thrown.Message != fixture.Thrown.Message || text != "" {
					t.Fatalf("display: text=%q error=%v; want %v", text, err, fixture.Thrown)
				}
			} else {
				want, formatErr := FormatValueExpression(fixture.Output, 80, 0)
				if formatErr != nil || err != nil || text != want {
					t.Fatalf("display: got %q err=%v; want %q err=%v", text, err, want, formatErr)
				}
			}
			if counter := session.runtime.ObjectHashState().Counter(); counter != fixture.Counter {
				t.Fatalf("object counter: got %d, want %d", counter, fixture.Counter)
			}
		})
	}
}

func TestREPLDomainDisplayAndDebugRespectCancellation(t *testing.T) {
	t.Parallel()
	session := NewREPLSession()
	expression, err := ParseExpression("0.to(1000000000000000000000000000000)")
	if err != nil {
		t.Fatal(err)
	}
	value, err := EvaluatePureExpression(context.Background(), expression, nil)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = session.FormatValueContext(canceled, value, 80); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled display: %v", err)
	}
	// A deterministic cancellation context stops an enormous domain after a
	// few checked operations, without a timer, goroutine, or materialization.
	ctx := &replFormattingCancellation{Context: context.Background(), remaining: 12}
	if _, err = session.FormatValueContext(ctx, value, 80); !errors.Is(err, context.Canceled) || ctx.remaining != 0 {
		t.Fatalf("cancellation during display: remaining=%d error=%v", ctx.remaining, err)
	}
	label, err := NewRuntimeValue(Expr{Kind: "str", Str: "huge"})
	if err != nil {
		t.Fatal(err)
	}
	ctx = &replFormattingCancellation{Context: WithRuntimeHooks(context.Background(), RuntimeHooks{DebugWriter: io.Discard}), remaining: 12}
	if _, err = EvalDebugBuiltin(ctx, []RuntimeValue{label, value}, session.runtime.ObjectHashState()); !errors.Is(err, context.Canceled) || ctx.remaining != 0 {
		t.Fatalf("cancellation during debug: remaining=%d error=%v", ctx.remaining, err)
	}
}

type replFormattingCancellation struct {
	context.Context
	remaining int
}

func (c *replFormattingCancellation) Err() error {
	if c.remaining > 0 {
		c.remaining--
	}
	if c.remaining == 0 {
		return context.Canceled
	}
	return nil
}
