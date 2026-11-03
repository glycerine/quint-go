package quint

import "context"

// SerializeRuntimeTraceContext converts retained states to the reference's
// display expressions. A single object-hash sequence covers the entire trace;
// passing nil starts a fresh sequence. Finite compact domains are enumerated
// with the caller's context, and serialization errors are returned unchanged.
func SerializeRuntimeTraceContext(ctx context.Context, states []RuntimeValue, objects *RuntimeObjectHashState) ([]Expr, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if objects == nil {
		objects = &RuntimeObjectHashState{}
	}
	expressions := make([]Expr, len(states))
	for i, state := range states {
		var err error
		expressions[i], err = SerializeRuntimeValueContext(ctx, state, objects)
		if err != nil {
			return nil, err
		}
	}
	return expressions, nil
}
