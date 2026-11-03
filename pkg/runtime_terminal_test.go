package quint

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glycerine/quint-go/pkg/terminal"
)

func TestRuntimeTerminalDebugUsesContextWidthAndOptionalStyles(t *testing.T) {
	t.Parallel()
	label, err := NewRuntimeValue(Expr{Kind: "str", Str: "label"})
	if err != nil {
		t.Fatal(err)
	}
	expr, err := ParseExpression("List(1,2,3)")
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(expr)
	if err != nil {
		t.Fatal(err)
	}
	level := 1
	ctx := terminal.WithOptions(context.Background(), terminal.Options{Width: 8, ColorLevel: &level})
	var out bytes.Buffer
	ctx = WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: &out})
	got, err := EvalDebugBuiltin(ctx, []RuntimeValue{label, value})
	if err != nil || !sameRuntimeObject(got, value) || out.String() != "> label [\n  \x1b[33m1\x1b[39m,\n  \x1b[33m2\x1b[39m,\n  \x1b[33m3\x1b[39m\n]\n" {
		t.Fatalf("debug=%q error=%v", out.String(), err)
	}
	out.Reset()
	ctx = WithRuntimeHooks(ctx, RuntimeHooks{DebugWriter: &out, DebugWidth: 80})
	if _, err = EvalDebugBuiltin(ctx, []RuntimeValue{label, value}); err != nil || strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("explicit width was lost: %q error=%v", out.String(), err)
	}
}

func TestREPLTerminalErrorPreservesCancelledContextMessage(t *testing.T) {
	t.Parallel()
	level := 1
	ctx, cancel := context.WithCancel(terminal.WithOptions(context.Background(), terminal.Options{ColorLevel: &level}))
	cancel()
	session := NewREPLSession()
	if got := session.RenderErrorContext(ctx, context.Canceled); got != context.Canceled.Error() {
		t.Fatalf("cancelled error=%q", got)
	}
	if got := session.RenderErrorContext(ctx, nil); got != "" {
		t.Fatalf("nil error=%q", got)
	}
	_, err := session.Evaluate(context.Background(), "missing")
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) {
		t.Fatalf("expected source diagnostic, got %v", err)
	}
	if got := session.RenderErrorContext(ctx, err); got != session.RenderError(err) || !strings.Contains(got, "QNT404") {
		t.Fatalf("primary excerpt should remain plain: %q", got)
	}
}
