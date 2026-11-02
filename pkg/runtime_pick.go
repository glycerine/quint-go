package quint

import (
	"context"
	"math/big"
)

// RuntimePickBounds returns the positional sampling bounds of an explicit
// finite set. A set needs one position, even when its elements are sets or
// other composite values: picking returns the stored element itself.
func RuntimePickBounds(value RuntimeValue) ([]Integer, error) {
	if !value.Valid() || value.expression.Kind != "app" || value.expression.Opcode != "Set" {
		return nil, Diagnostic{Code: "QNT501", Message: "Picking requires an explicit finite Set value"}
	}
	entries, err := runtimeMap(value)
	if err != nil {
		return nil, err
	}
	return []Integer{IntegerFromBig(new(big.Int).SetUint64(uint64(entries.Len())))}, nil
}

// PickRuntimePosition selects from an explicit set or compact domain. Explicit
// sets use the first position and ignore extras; a nonpositive position selects
// the first element. Compact domains use their recursive positional protocol.
// Returned stored elements retain their identity and nested collection history.
func PickRuntimePosition(ctx context.Context, value RuntimeValue, positions []Integer) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if value.domain != nil {
		return PickRuntimePositionContext(ctx, value, positions)
	}
	if _, err := RuntimePickBounds(value); err != nil {
		return RuntimeValue{}, err
	}
	if len(positions) == 0 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Internal error: too few positions. Report a bug."}
	}
	entries, err := runtimeEntries(value)
	if err != nil {
		return RuntimeValue{}, err
	}
	index, one := positions[0].Big(), big.NewInt(1)
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		if index.Sign() <= 0 {
			return entry.Value, nil
		}
		index.Sub(index, one)
	}
	return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Index out of bounds"}
}

// PickRuntimeValue samples an explicit finite set or compact domain using the
// caller's RNG.
// Empty sets return the reference picking diagnostic without advancing RNG;
// nondeterministic bindings handle their empty-set false result separately.
func PickRuntimeValue(ctx context.Context, value RuntimeValue, rng *RNG) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if value.domain != nil {
		return PickRuntimeValueContext(ctx, value, rng)
	}
	bounds, err := RuntimePickBounds(value)
	if err != nil {
		return RuntimeValue{}, err
	}
	if bounds[0].Big().Sign() == 0 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Index out of bounds"}
	}
	if rng == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Picking requires an RNG"}
	}
	position, err := rng.Next(bounds[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	return PickRuntimePosition(ctx, value, []Integer{position})
}
