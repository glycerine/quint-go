package quint

import (
	"context"
	"fmt"
	"io"
	"os"
)

// RuntimeHooks selects the output destination and layout for q::debug calls.
// The default destination is stdout; a nonpositive width uses 80 columns.
// Hooks belong to one evaluation context and do not change global output.
type RuntimeHooks struct {
	DebugWriter io.Writer
	DebugWidth  int
}

type runtimeHooksKey struct{}

// WithRuntimeHooks gives evaluations caller-owned debug output settings.
// Pass io.Discard as DebugWriter to suppress output explicitly.
func WithRuntimeHooks(ctx context.Context, hooks RuntimeHooks) context.Context {
	return context.WithValue(ctx, runtimeHooksKey{}, hooks)
}

// EvalDebugBuiltin prints the reference q::debug prefix, label, and pretty value,
// then returns the exact value object. Serialization can share an evaluator's
// object-hash state so debug calls participate in its display history.
func EvalDebugBuiltin(ctx context.Context, args []RuntimeValue, states ...*RuntimeObjectHashState) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if len(args) != 2 {
		return fail("q::debug requires 2 arguments")
	}
	for _, value := range args {
		if !value.Valid() {
			return fail("Invalid runtime value")
		}
	}
	var state *RuntimeObjectHashState
	if len(states) > 0 {
		state = states[0]
	}
	expression, err := SerializeRuntimeValueContext(ctx, args[1], state)
	if err != nil {
		return RuntimeValue{}, err
	}
	hooks, _ := ctx.Value(runtimeHooksKey{}).(RuntimeHooks)
	if hooks.DebugWidth <= 0 {
		hooks.DebugWidth = 80
	}
	formatted, err := FormatValueExpression(expression, hooks.DebugWidth, 0)
	if err != nil {
		return RuntimeValue{}, err
	}
	if args[0].expression.Kind != "str" {
		return fail("Expected a string value")
	}
	writer := hooks.DebugWriter
	if writer == nil {
		writer = os.Stdout
	}
	if _, err = fmt.Fprintf(writer, "> %s %s\n", args[0].expression.Str, formatted); err != nil {
		return RuntimeValue{}, err
	}
	return args[1], nil
}
