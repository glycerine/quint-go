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
)

// RuntimeDomain is an immutable explicit finite set, inclusive integer
// interval, or Int/Nat domain. Compact domains do not materialize their elements
// when constructed, queried for membership/cardinality, or sampled. Its zero
// value is invalid. RuntimeValue integration is separate from this domain API.
type RuntimeDomain struct {
	kind        runtimeDomainKind
	first, last Integer
	explicit    RuntimeValue
}

// RuntimeDomainFromValue retains an explicit finite set and its traversal.
func RuntimeDomainFromValue(value RuntimeValue) (RuntimeDomain, error) {
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
	return d.kind >= runtimeExplicitDomain && d.kind <= runtimeNaturalDomain
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
	if d.kind == runtimeExplicitDomain {
		normal, err := runtimeNormalForm(value)
		if err != nil {
			return false, err
		}
		entries, err := runtimeMap(d.explicit)
		if err != nil {
			return false, err
		}
		_, found, err := entries.Lookup(normal)
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
	contained := true
	err := d.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		var err error
		contained, err = other.Contains(ctx, value)
		return contained, err
	})
	return contained, err
}

// Equal compares domains extensionally across explicit sets and intervals,
// avoiding interval materialization. Int/Nat only equal their own infinite kind.
// Private reference hashes may differ before normalization, as upstream does.
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
		return d.explicit.Equal(other.explicit), nil
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
	if err := d.validate(ctx); err != nil {
		return RuntimeValue{}, err
	}
	if len(positions) == 0 {
		return RuntimeValue{}, RuntimeDomainException{Name: "AssertionError", Message: "Internal error: too few positions. Report a bug."}
	}
	if d.kind == runtimeExplicitDomain {
		return PickRuntimePosition(ctx, d.explicit, positions)
	}
	position := positions[0].Big()
	if d.kind == runtimeIntervalDomain {
		if d.last.Big().Cmp(d.first.Big()) < 0 {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Index out of bounds"}
		}
		position.Add(position, d.first.Big())
	} else if d.kind == runtimeNaturalDomain {
		position.Abs(position)
	}
	return NewRuntimeValue(Expr{Kind: "int", Int: IntegerFromBig(position)})
}

// Sample draws one position from the caller-owned RNG. Infinite positions use
// the reference nondeterministic range [-2^255,2^255); Nat takes the absolute
// position when picking. Empty domains and invalid inputs do not consume RNG.
func (d RuntimeDomain) Sample(ctx context.Context, rng *RNG) (RuntimeValue, error) {
	bounds, err := d.Bounds(ctx)
	if err != nil {
		return RuntimeValue{}, err
	}
	bound := bounds[0]
	if bound.Finite && bound.Value.Big().Sign() == 0 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Index out of bounds"}
	}
	if rng == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Picking requires an RNG"}
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
	return d.Pick(ctx, []Integer{position})
}

// Materialize converts a finite domain to the normalized explicit Set form.
// Explicit domains retain their underlying tree; intervals are enumerated in
// ascending numeric order before the persistent mkSet normalization boundary.
func (d RuntimeDomain) Materialize(ctx context.Context) (RuntimeValue, error) {
	if err := d.validate(ctx); err != nil {
		return RuntimeValue{}, err
	}
	if d.kind == runtimeExplicitDomain {
		return runtimeNormalForm(d.explicit)
	}
	values := []RuntimeValue{}
	err := d.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		values = append(values, value)
		return true, nil
	})
	if err != nil {
		return RuntimeValue{}, err
	}
	return newRuntimeComposite("Set", values)
}

// Expression returns the reference's direct domain display. Intervals preserve
// numeric enumeration order; that display differs from a normalized Set's
// string-collation order. Int/Nat remain built-in names.
func (d RuntimeDomain) Expression(ctx context.Context) (Expr, error) {
	if err := d.validate(ctx); err != nil {
		return Expr{}, err
	}
	if d.kind == runtimeExplicitDomain {
		return d.explicit.Expression(), nil
	}
	if d.infinite() {
		return Expr{Kind: "name", Name: d.infiniteName()}, nil
	}
	result := Expr{Kind: "app", Opcode: "Set", Args: []Expr{}}
	err := d.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		result.Args = append(result.Args, value.Expression())
		return true, nil
	})
	return result, err
}

// referenceHashCode is the compact class's raw upstream hash, before
// Immutable.hash's signed-small-integer conversion. It is not a Go structural
// hash: finite domains must normalize before use as collection keys/elements.
func (d RuntimeDomain) referenceHashCode() (int64, error) {
	if !d.Valid() {
		return 0, fmt.Errorf("reference hashing requires a valid runtime domain")
	}
	switch d.kind {
	case runtimeExplicitDomain:
		return runtimeReferenceHashCode(d.explicit.expression)
	case runtimeIntervalDomain:
		n := new(big.Int).Add(d.first.Big(), d.last.Big())
		n.Mod(n, new(big.Int).Lsh(big.NewInt(1), 32))
		return n.Int64(), nil
	case runtimeNaturalDomain:
		return 9007199254740990, nil
	default:
		return 9007199254740991, nil
	}
}
