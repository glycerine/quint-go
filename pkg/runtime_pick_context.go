package quint

import "context"

// RuntimePickBoundsContext returns checked positional bounds for an explicit
// set or compact domain. Infinite bounds carry Finite=false. Bounds describe
// immediate factors, preserving the reference's nested-product behavior.
func RuntimePickBoundsContext(ctx context.Context, value RuntimeValue) ([]RuntimeBound, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	domain, err := RuntimeDomainFromValue(value)
	if err != nil {
		return nil, err
	}
	return domain.Bounds(ctx)
}

// PickRuntimePositionContext picks from a checked set-like value. Nested
// domains share one position cursor; positions are not changed. Enumeration,
// missing-position exceptions, and cancellation are returned unchanged.
func PickRuntimePositionContext(ctx context.Context, value RuntimeValue, positions []Integer) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	domain, err := RuntimeDomainFromValue(value)
	if err != nil {
		return RuntimeValue{}, err
	}
	return domain.Pick(ctx, positions)
}

// PickRuntimeValueContext samples every bound using the caller's RNG. Infinite
// positions use [-2^255,2^255); Nat picks their absolute values. Finite zero bounds
// fail without drawing, while a domain with no positional bounds needs no RNG.
func PickRuntimeValueContext(ctx context.Context, value RuntimeValue, rng *RNG) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	domain, err := RuntimeDomainFromValue(value)
	if err != nil {
		return RuntimeValue{}, err
	}
	return domain.Sample(ctx, rng)
}
