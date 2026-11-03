package quint

import (
	"context"
	"fmt"
)

// DecodeNormalizedITFTrace implements upstream ofItfNormalized's concrete-state
// runtime round trip while retaining the original bytes and metadata. One fresh
// object-hash sequence covers all states, including otherwise tied display order.
// NormalizeValue remains the deterministic structural form for individual values.
func DecodeNormalizedITFTrace(data []byte) (ITFTrace, error) {
	return DecodeNormalizedITFTraceContext(context.Background(), data, nil)
}

// DecodeNormalizedITFTraceContext adds cooperative cancellation and an optional
// caller-owned object-hash sequence. Nil starts a fresh sequence. Use a supplied
// state sequentially, just as with SerializeRuntimeTraceContext. JSON decoding
// and concrete value construction are synchronous; cancellation is checked around
// those phases and during serialization. The source rejects symbolic ITF names;
// retained symbolic RuntimeValues use SerializeRuntimeTraceContext instead.
// Display sorting retains the runtime's pinned en-US collation limitations.
func DecodeNormalizedITFTraceContext(ctx context.Context, data []byte, objects *RuntimeObjectHashState) (ITFTrace, error) {
	if err := ctx.Err(); err != nil {
		return ITFTrace{}, err
	}
	trace, err := DecodeITFTrace(data)
	if err != nil {
		return ITFTrace{}, err
	}
	// Upstream converts every state before serializing any of them. Preserve
	// both that error precedence and the shared serialization object history.
	values := make([]RuntimeValue, len(trace.States))
	for i, state := range trace.States {
		if err := ctx.Err(); err != nil {
			return ITFTrace{}, err
		}
		values[i], err = NewRuntimeValue(state)
		if err != nil {
			return ITFTrace{}, fmt.Errorf("normalizing ITF state %d: %w", i, err)
		}
	}
	if objects == nil {
		objects = &RuntimeObjectHashState{}
	}
	for i, value := range values {
		trace.States[i], err = SerializeRuntimeValueContext(ctx, value, objects)
		if err != nil {
			return ITFTrace{}, fmt.Errorf("normalizing ITF state %d: %w", i, err)
		}
	}
	if err := ctx.Err(); err != nil {
		return ITFTrace{}, err
	}
	return trace, nil
}
