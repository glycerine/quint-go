package quint

import (
	"context"
	"errors"
)

// EvalMapSetBy replaces an existing key's value by applying a closure to its
// current value. Missing keys fail before invoking the closure.
func EvalMapSetBy(ctx context.Context, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if len(args) != 3 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "setBy requires 3 arguments"}
	}
	current, err := EvalMapBuiltin("get", args[:2])
	if err != nil {
		var d Diagnostic
		if errors.As(err, &d) && d.Code == "QNT507" {
			// Upstream interpolates the runtime key object rather than its IR string.
			return RuntimeValue{}, Diagnostic{Code: "QNT507", Message: "Called 'setBy' with a non- existing key [object Object]"}
		}
		return RuntimeValue{}, err
	}
	updated, err := args[2].Apply(ctx, []RuntimeValue{current})
	if err != nil {
		return RuntimeValue{}, err
	}
	return EvalMapBuiltin("set", []RuntimeValue{args[0], args[1], updated})
}
