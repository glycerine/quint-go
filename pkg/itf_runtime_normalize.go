package quint

import "context"

// NormalizeRuntimeValueContext implements the reference runtime's normalForm
// operation on a retained value. It materializes finite symbolic domains and
// recursively normalizes lists, tuples, records, and maps. Variants are opaque:
// their retained payload is not normalized, even when it contains a domain.
//
// Normalization preserves the input's language value and retained domains. The
// result may share immutable payloads, collection trees, closures, and reference
// hash memo history with the input; it is not a fresh transport graph. Memo
// history can advance during operations. Use a shared execution owner
// sequentially.
//
// Finite enumeration has no cardinality cap and can allocate its full result.
// Cancellation is cooperative: entry and symbolic enumeration check ctx, while
// concrete copying and individual operations may finish before checking again.
// Invalid values and enumeration errors are returned unchanged. Normalization
// does not serialize the result or guarantee that it can be encoded as ITF.
func NormalizeRuntimeValueContext(ctx context.Context, value RuntimeValue) (RuntimeValue, error) {
	return runtimeNormalFormContext(ctx, value)
}
