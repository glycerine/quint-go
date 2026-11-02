package quint

import (
	"context"
	"os"
)

type runtimeNondetOptionsKey struct{}

type runtimeNondetConfiguration struct {
	options RuntimeNondetOptions
	err     error
}

// The reference initializes its retry constant when the nondet module loads.
// Snapshot the process environment once as well, retaining a parse error for
// checked evaluation instead of making a Go package import panic.
var runtimeNondetProcessConfiguration = func() runtimeNondetConfiguration {
	text, present := os.LookupEnv("QUINT_RETRY_NONDET_SMALLER_THAN")
	options, err := runtimeNondetOptionsFromEnvironment(text, present)
	return runtimeNondetConfiguration{options: options, err: err}
}()

func runtimeNondetOptionsFromEnvironment(text string, present bool) (RuntimeNondetOptions, error) {
	if !present {
		text = "100"
	}
	limit, err := ParseSeed(text)
	if err != nil {
		return RuntimeNondetOptions{}, RuntimeDomainException{Name: "SyntaxError", Message: "Cannot convert " + text + " to a BigInt"}
	}
	return RuntimeNondetOptions{RetrySmallerThan: limit}, nil
}

// WithRuntimeNondetOptions overrides nondeterministic retries for one evaluation
// context. It also applies to nested bindings. Without an override, EvalNondet
// uses the process environment captured when the package initialized.
func WithRuntimeNondetOptions(ctx context.Context, options RuntimeNondetOptions) context.Context {
	return context.WithValue(ctx, runtimeNondetOptionsKey{}, options)
}

func runtimeNondetOptionsForContext(ctx context.Context) (RuntimeNondetOptions, error) {
	if options, ok := ctx.Value(runtimeNondetOptionsKey{}).(RuntimeNondetOptions); ok {
		return options, nil
	}
	return runtimeNondetProcessConfiguration.options, runtimeNondetProcessConfiguration.err
}
