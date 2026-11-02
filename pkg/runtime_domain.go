package quint

import (
	"context"
	"fmt"
	"math/big"
)

// RuntimeBound distinguishes a finite positional sampling bound from an
// unbounded integer choice. Value is meaningful only when Finite is true.
type RuntimeBound struct {
	Finite bool
	Value  Integer
}

// RuntimeDomainException represents an exception thrown by the reference
// runtime, rather than a language diagnostic returned by an operation.
type RuntimeDomainException struct {
	Name    string
	Message string
}

func (e RuntimeDomainException) Error() string { return e.Message }

type runtimeDomainKind uint8

const (
	runtimeInvalidDomain runtimeDomainKind = iota
	runtimeExplicitDomain
	runtimeIntervalDomain
	runtimeIntegerDomain
	runtimeNaturalDomain
	runtimeProductDomain
	runtimePowersetDomain
	runtimeMapSetDomain
)

// RuntimeDomain is an immutable explicit finite set, inclusive integer interval,
// Int/Nat domain, product, powerset, or map set. Compact domains do not materialize their elements
// when constructed, queried for membership/cardinality, or sampled. Its zero
// value is invalid. RuntimeValueFromDomain retains this compact representation.
type RuntimeDomain struct {
	kind        runtimeDomainKind
	first, last Integer
	explicit    RuntimeValue
	factors     []RuntimeDomain
	base        *RuntimeDomain
	domain      *RuntimeDomain
	rangeSet    *RuntimeDomain
}

// RuntimeDomainFromValue retains an explicit finite set and its traversal, or
// unwraps a compact runtime domain without enumerating it.
func RuntimeDomainFromValue(value RuntimeValue) (RuntimeDomain, error) {
	if value.Valid() && value.domain != nil {
		return *value.domain, nil
	}
	if !value.Valid() || value.expression.Kind != "app" || value.expression.Opcode != "Set" {
		return RuntimeDomain{}, Diagnostic{Code: "QNT501", Message: "Expected an explicit finite Set value"}
	}
	if _, err := runtimeMap(value); err != nil {
		return RuntimeDomain{}, err
	}
	return RuntimeDomain{kind: runtimeExplicitDomain, explicit: value}, nil
}

// NewRuntimeInterval constructs an inclusive interval. All empty intervals use
// the reference representation [1,0], independently of their input endpoints.
func NewRuntimeInterval(first, last Integer) RuntimeDomain {
	if last.Big().Cmp(first.Big()) < 0 {
		first = IntegerFromBig(big.NewInt(1))
		last = Integer{}
	}
	return RuntimeDomain{kind: runtimeIntervalDomain, first: first, last: last}
}

// NewRuntimeInfiniteSet constructs the built-in Int or Nat domain.
func NewRuntimeInfiniteSet(kind string) (RuntimeDomain, error) {
	switch kind {
	case "Int":
		return RuntimeDomain{kind: runtimeIntegerDomain}, nil
	case "Nat":
		return RuntimeDomain{kind: runtimeNaturalDomain}, nil
	default:
		return RuntimeDomain{}, Diagnostic{Code: "QNT501", Message: "Unknown infinite set " + kind}
	}
}

func (d RuntimeDomain) Valid() bool {
	return d.kind >= runtimeExplicitDomain && d.kind <= runtimeMapSetDomain
}

func (d RuntimeDomain) validate(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !d.Valid() {
		return Diagnostic{Code: "QNT501", Message: "Invalid runtime domain"}
	}
	return nil
}

func (d RuntimeDomain) infiniteName() string {
	if d.kind == runtimeNaturalDomain {
		return "Nat"
	}
	return "Int"
}

func (d RuntimeDomain) infinite() bool {
	return d.kind == runtimeIntegerDomain || d.kind == runtimeNaturalDomain
}

// Cardinality returns an arbitrary-precision finite size or QNT514 for Int/Nat.
func (d RuntimeDomain) Cardinality(ctx context.Context) (Integer, error) {
	if err := d.validate(ctx); err != nil {
		return Integer{}, err
	}
	switch d.kind {
	case runtimeExplicitDomain:
		entries, err := runtimeMap(d.explicit)
		if err != nil {
			return Integer{}, err
		}
		return IntegerFromBig(new(big.Int).SetUint64(uint64(entries.Len()))), nil
	case runtimeIntervalDomain:
		size := new(big.Int).Sub(d.last.Big(), d.first.Big())
		return IntegerFromBig(size.Add(size, big.NewInt(1))), nil
	case runtimeProductDomain, runtimePowersetDomain, runtimeMapSetDomain:
		return d.compoundCardinality(ctx)
	default:
		return Integer{}, Diagnostic{Code: "QNT514", Message: "Cardinality of " + d.infiniteName() + " is infinite"}
	}
}

// Bounds returns a fresh positional-bound slice. These domains need one
// position, including an empty finite set; Int/Nat use an unbounded position.
func (d RuntimeDomain) Bounds(ctx context.Context) ([]RuntimeBound, error) {
	if err := d.validate(ctx); err != nil {
		return nil, err
	}
	if d.infinite() {
		return []RuntimeBound{{Finite: false}}, nil
	}
	if d.compound() {
		return d.compoundBounds(ctx)
	}
	size, err := d.Cardinality(ctx)
	if err != nil {
		return nil, err
	}
	return []RuntimeBound{{Finite: true, Value: size}}, nil
}

// Contains tests integer-domain membership without enumeration. Explicit sets
// retain the existing concrete collection equality/normal-form boundary.
func (d RuntimeDomain) Contains(ctx context.Context, value RuntimeValue) (bool, error) {
	if err := d.validate(ctx); err != nil {
		return false, err
	}
	if !value.Valid() {
		return false, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
	}
	if d.compound() {
		return d.compoundContains(ctx, value)
	}
	if d.kind == runtimeExplicitDomain {
		normal, err := runtimeKeyNormalFormContext(ctx, value)
		if err != nil {
			return false, err
		}
		entries, err := runtimeMap(d.explicit)
		if err != nil {
			return false, err
		}
		_, found, err := entries.LookupContext(ctx, normal)
		return found, err
	}
	if value.expression.Kind != "int" {
		return false, nil
	}
	n := value.expression.Int.Big()
	if d.infinite() {
		return d.kind == runtimeIntegerDomain || n.Sign() >= 0, nil
	}
	return d.first.Big().Cmp(n) <= 0 && n.Cmp(d.last.Big()) <= 0, nil
}

// ForEach visits retained explicit elements or ascending interval integers.
// Returning false stops iteration. The callback and context can bound work
// without materializing the whole interval; Int/Nat are non-enumerable.
func (d RuntimeDomain) ForEach(ctx context.Context, yield func(RuntimeValue) (bool, error)) error {
	if err := d.validate(ctx); err != nil {
		return err
	}
	if yield == nil {
		return Diagnostic{Code: "QNT501", Message: "Runtime domain iteration requires a callback"}
	}
	if d.infinite() {
		return RuntimeDomainException{Name: "Error", Message: "Infinite set " + d.infiniteName() + " is non-enumerable"}
	}
	if d.compound() {
		return d.compoundForEach(ctx, yield)
	}
	if d.kind == runtimeExplicitDomain {
		entries, err := runtimeEntries(d.explicit)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			more, err := yield(entry.Value)
			if err != nil || !more {
				return err
			}
		}
		return nil
	}
	last, one := d.last.Big(), big.NewInt(1)
	for next := d.first.Big(); next.Cmp(last) <= 0; next.Add(next, one) {
		if err := ctx.Err(); err != nil {
			return err
		}
		value, err := NewRuntimeValue(Expr{Kind: "int", Int: IntegerFromBig(next)})
		if err != nil {
			return err
		}
		more, err := yield(value)
		if err != nil || !more {
			return err
		}
	}
	return nil
}

// Subset reproduces the reference's optimized interval and infinite-domain
// cases. In particular its [1,0] empty interval endpoint test is preserved.
func (d RuntimeDomain) Subset(ctx context.Context, other RuntimeDomain) (bool, error) {
	if err := d.validate(ctx); err != nil {
		return false, err
	}
	if err := other.validate(ctx); err != nil {
		return false, err
	}
	if d.infinite() {
		return other.infinite() && (d.kind != runtimeIntegerDomain || other.kind != runtimeNaturalDomain), nil
	}
	if d.kind == runtimeIntervalDomain && other.kind == runtimeIntervalDomain {
		return d.first.Big().Cmp(other.first.Big()) >= 0 && d.last.Big().Cmp(other.last.Big()) <= 0, nil
	}
	if d.compound() && d.kind == other.kind {
		return d.compoundSubset(ctx, other)
	}
	contained := true
	err := d.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		var err error
		contained, err = other.Contains(ctx, value)
		return contained, err
	})
	return contained, err
}

// Equal follows the upstream domain classes, avoiding interval materialization.
// Int/Nat only equal their own infinite kind. Explicit sets preserve source key
// hash history, which can make equality asymmetric for symbolic keys; use
// RuntimeValue.EqualContext for the symmetric public Go comparison.
func (d RuntimeDomain) Equal(ctx context.Context, other RuntimeDomain) (bool, error) {
	if err := d.validate(ctx); err != nil {
		return false, err
	}
	if err := other.validate(ctx); err != nil {
		return false, err
	}
	if d.infinite() || other.infinite() {
		return d.kind == other.kind, nil
	}
	if d.kind == runtimeIntervalDomain && other.kind == runtimeIntervalDomain {
		return d.first == other.first && d.last == other.last, nil
	}
	if d.kind == runtimeExplicitDomain && other.kind == runtimeExplicitDomain {
		return runtimeSourceEqualContext(ctx, d.explicit, other.explicit)
	}
	if d.compound() || other.compound() {
		if d.kind == other.kind {
			return d.compoundEqual(ctx, other)
		}
		left, err := d.Materialize(ctx)
		if err != nil {
			return false, err
		}
		right, err := other.Materialize(ctx)
		if err != nil {
			return false, err
		}
		return runtimeSourceEqualContext(ctx, left, right)
	}
	leftSize, err := d.Cardinality(ctx)
	if err != nil {
		return false, err
	}
	rightSize, err := other.Cardinality(ctx)
	if err != nil || leftSize != rightSize {
		return false, err
	}
	if d.kind == runtimeExplicitDomain {
		return d.Subset(ctx, other)
	}
	return other.Subset(ctx, d)
}

// Pick selects the first position and ignores additional positions. As in the
// reference, a nonempty interval returns first+position even outside its bounds.
func (d RuntimeDomain) Pick(ctx context.Context, positions []Integer) (RuntimeValue, error) {
	value, _, err := d.PickWithPositions(ctx, positions)
	return value, err
}

// Sample draws one position from the caller-owned RNG. Infinite positions use
// the reference nondeterministic range [-2^255,2^255); Nat takes the absolute
// position when picking. Empty domains and invalid inputs do not consume RNG.
func (d RuntimeDomain) Sample(ctx context.Context, rng *RNG) (RuntimeValue, error) {
	bounds, err := d.Bounds(ctx)
	if err != nil {
		return RuntimeValue{}, err
	}
	for _, bound := range bounds {
		if bound.Finite && bound.Value.Big().Sign() == 0 {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Index out of bounds"}
		}
	}
	if rng == nil && len(bounds) > 0 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Picking requires an RNG"}
	}
	positions := make([]Integer, len(bounds))
	for i, bound := range bounds {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		limit := bound.Value
		if !bound.Finite {
			limit = IntegerFromBig(new(big.Int).Lsh(big.NewInt(1), 256))
		}
		position, err := rng.Next(limit)
		if err != nil {
			return RuntimeValue{}, err
		}
		if !bound.Finite {
			n := position.Big()
			n.Sub(n, new(big.Int).Lsh(big.NewInt(1), 255))
			position = IntegerFromBig(n)
		}
		positions[i] = position
	}
	return d.Pick(ctx, positions)
}

// Materialize converts a finite domain to the normalized explicit Set form.
// Explicit domains retain their underlying tree; intervals are enumerated in
// ascending numeric order before the persistent mkSet normalization boundary.
func (d RuntimeDomain) Materialize(ctx context.Context) (RuntimeValue, error) {
	if err := d.validate(ctx); err != nil {
		return RuntimeValue{}, err
	}
	if d.kind == runtimeExplicitDomain {
		return runtimeNormalFormContext(ctx, d.explicit)
	}
	values := []RuntimeValue{}
	err := d.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		values = append(values, value)
		return true, nil
	})
	if err != nil {
		return RuntimeValue{}, err
	}
	return NewRuntimeCompositeContext(ctx, "Set", values)
}

// Expression returns the reference's direct domain display. Intervals preserve
// numeric enumeration order; that display differs from a normalized Set's
// string-collation order. Int/Nat remain built-in names.
func (d RuntimeDomain) Expression(ctx context.Context) (Expr, error) {
	return d.Serialize(ctx, nil)
}

// Serialize shares the reference object-hash sequence when state is non-nil.
// Compound displays preserve enumeration order and serialize retained children.
func (d RuntimeDomain) Serialize(ctx context.Context, state *RuntimeObjectHashState) (Expr, error) {
	if err := d.validate(ctx); err != nil {
		return Expr{}, err
	}
	if d.kind == runtimeExplicitDomain {
		return SerializeRuntimeValueContext(ctx, d.explicit, state)
	}
	if d.infinite() {
		return Expr{Kind: "name", Name: d.infiniteName()}, nil
	}
	if state == nil {
		state = &RuntimeObjectHashState{}
	}
	result := Expr{Kind: "app", Opcode: "Set", Args: []Expr{}}
	err := d.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		expression, err := SerializeRuntimeValueContext(ctx, value, state)
		if err == nil {
			result.Args = append(result.Args, expression)
		}
		return true, err
	})
	return result, err
}

// referenceHashCode is the compact class's raw upstream hash, before
// Immutable.hash's signed-small-integer conversion. It is not a Go structural
// hash: finite domains must normalize before use as collection keys/elements.
func (d RuntimeDomain) referenceHashCode() (float64, error) {
	if !d.Valid() {
		return 0, fmt.Errorf("reference hashing requires a valid runtime domain")
	}
	switch d.kind {
	case runtimeExplicitDomain:
		code, err := runtimeReferenceHashCode(d.explicit.expression)
		return float64(code), err
	case runtimeIntervalDomain:
		n := new(big.Int).Add(d.first.Big(), d.last.Big())
		n.Mod(n, new(big.Int).Lsh(big.NewInt(1), 32))
		return float64(n.Int64()), nil
	case runtimeNaturalDomain:
		return 9007199254740990, nil
	case runtimeProductDomain, runtimePowersetDomain, runtimeMapSetDomain:
		return d.compoundReferenceHashCode()
	default:
		return 9007199254740991, nil
	}
}
