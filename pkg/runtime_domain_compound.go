package quint

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strings"
)

// NewRuntimeProduct retains an ordered, independently owned list of factors.
// Construction does not enumerate factors, including infinite ones.
func NewRuntimeProduct(factors []RuntimeDomain) (RuntimeDomain, error) {
	for _, factor := range factors {
		if !factor.Valid() {
			return RuntimeDomain{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime domain"}
		}
	}
	return RuntimeDomain{kind: runtimeProductDomain, factors: append([]RuntimeDomain{}, factors...)}, nil
}

// NewRuntimePowerset retains an immutable base without enumerating its subsets.
func NewRuntimePowerset(base RuntimeDomain) (RuntimeDomain, error) {
	if !base.Valid() {
		return RuntimeDomain{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime domain"}
	}
	return RuntimeDomain{kind: runtimePowersetDomain, base: &base}, nil
}

// NewRuntimeMapSet retains immutable domain and range representations.
func NewRuntimeMapSet(domain, rangeSet RuntimeDomain) (RuntimeDomain, error) {
	if !domain.Valid() || !rangeSet.Valid() {
		return RuntimeDomain{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime domain"}
	}
	return RuntimeDomain{kind: runtimeMapSetDomain, domain: &domain, rangeSet: &rangeSet}, nil
}

func (d RuntimeDomain) compound() bool {
	return d.kind >= runtimeProductDomain && d.kind <= runtimeMapSetDomain
}

func runtimeDomainMergedErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	messages := make([]string, len(errs))
	for i, err := range errs {
		messages[i] = err.Error()
	}
	return Diagnostic{Code: "QNT501", Message: strings.Join(messages, "\n")}
}

func runtimeDomainIsDiagnostic(err error) bool {
	var diagnostic Diagnostic
	return errors.As(err, &diagnostic)
}

// V8 12.4, used by the saved Node reference, limits a BigInt to 2^30 bits.
// Preflight oversized exponents before allocating their result. The loop also
// checks cancellation between exponentiation steps.
func runtimeDomainPower(ctx context.Context, base, exponent *big.Int) (Integer, error) {
	if err := ctx.Err(); err != nil {
		return Integer{}, err
	}
	const maxBits = 1 << 30
	if exponent.Sign() == 0 {
		return IntegerFromBig(big.NewInt(1)), nil
	}
	if base.Sign() == 0 {
		return Integer{}, nil
	}
	if base.Cmp(big.NewInt(1)) == 0 {
		return IntegerFromBig(big.NewInt(1)), nil
	}
	tooLarge := func() (Integer, error) {
		return Integer{}, RuntimeDomainException{Name: "RangeError", Message: "Maximum BigInt size exceeded"}
	}
	if exponent.BitLen() > 30 || exponent.Cmp(big.NewInt(maxBits)) >= 0 {
		return tooLarge()
	}
	exponentFloat := float64(exponent.Int64())
	baseFloat, _ := new(big.Float).SetInt(base).Float64()
	if math.Log2(baseFloat)*exponentFloat >= maxBits {
		return tooLarge()
	}
	remaining, factor, result := new(big.Int).Set(exponent), new(big.Int).Set(base), big.NewInt(1)
	for remaining.Sign() > 0 {
		if err := ctx.Err(); err != nil {
			return Integer{}, err
		}
		if remaining.Bit(0) != 0 {
			result.Mul(result, factor)
		}
		remaining.Rsh(remaining, 1)
		if remaining.Sign() > 0 {
			factor.Mul(factor, factor)
		}
	}
	return IntegerFromBig(result), nil
}

func (d RuntimeDomain) compoundCardinality(ctx context.Context) (Integer, error) {
	if d.kind == runtimePowersetDomain {
		size, err := d.base.Cardinality(ctx)
		if err != nil {
			return Integer{}, err
		}
		return runtimeDomainPower(ctx, big.NewInt(2), size.Big())
	}
	factors := d.factors
	if d.kind == runtimeMapSetDomain {
		// mergeInMany evaluates range before domain and preserves error order.
		factors = []RuntimeDomain{*d.rangeSet, *d.domain}
	}
	cards := make([]Integer, len(factors))
	errs := []error{}
	for i, factor := range factors {
		card, err := factor.Cardinality(ctx)
		if err != nil {
			if !runtimeDomainIsDiagnostic(err) {
				return Integer{}, err
			}
			errs = append(errs, err)
		}
		cards[i] = card
	}
	if err := runtimeDomainMergedErrors(errs); err != nil {
		return Integer{}, err
	}
	if d.kind == runtimeMapSetDomain {
		return runtimeDomainPower(ctx, cards[0].Big(), cards[1].Big())
	}
	result := big.NewInt(1)
	for _, card := range cards {
		result.Mul(result, card.Big())
	}
	return IntegerFromBig(result), nil
}

func runtimeDomainBound(ctx context.Context, domain RuntimeDomain) (RuntimeBound, error) {
	size, err := domain.Cardinality(ctx)
	if runtimeDomainIsDiagnostic(err) {
		return RuntimeBound{}, nil
	}
	return RuntimeBound{Finite: err == nil, Value: size}, err
}

func (d RuntimeDomain) compoundBounds(ctx context.Context) ([]RuntimeBound, error) {
	if d.kind == runtimeProductDomain {
		bounds := make([]RuntimeBound, len(d.factors))
		for i, factor := range d.factors {
			bound, err := runtimeDomainBound(ctx, factor)
			if err != nil {
				return nil, err
			}
			bounds[i] = bound
		}
		return bounds, nil
	}
	if d.kind == runtimePowersetDomain {
		bound, err := runtimeDomainBound(ctx, d)
		return []RuntimeBound{bound}, err
	}
	size, err := d.domain.Cardinality(ctx)
	if err != nil && !runtimeDomainIsDiagnostic(err) {
		return nil, err
	}
	if err != nil || size.Big().Cmp(big.NewInt(9007199254740991)) > 0 {
		return nil, RuntimeDomainException{Name: "AssertionError", Message: "Domain size is over 9007199254740991"}
	}
	if size.Big().Cmp(new(big.Int).Lsh(big.NewInt(1), 32)) >= 0 {
		return nil, RuntimeDomainException{Name: "RangeError", Message: "Invalid array length"}
	}
	bound, err := runtimeDomainBound(ctx, *d.rangeSet)
	if err != nil {
		return nil, err
	}
	bounds := make([]RuntimeBound, int(size.Big().Int64()))
	for i := range bounds {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		bounds[i] = bound
	}
	return bounds, nil
}

// ContainsDomain tests membership of a set-like domain value. Powerset
// membership iterates the candidate rather than using optimized Subset rules.
func (d RuntimeDomain) ContainsDomain(ctx context.Context, element RuntimeDomain) (bool, error) {
	if err := d.validate(ctx); err != nil {
		return false, err
	}
	if err := element.validate(ctx); err != nil {
		return false, err
	}
	if d.kind == runtimePowersetDomain {
		contained := true
		err := element.ForEach(ctx, func(value RuntimeValue) (bool, error) {
			var err error
			contained, err = d.base.Contains(ctx, value)
			return contained, err
		})
		return contained, err
	}
	if d.kind != runtimeExplicitDomain {
		return false, nil
	}
	value, err := element.Materialize(ctx)
	if err != nil {
		return false, err
	}
	return d.Contains(ctx, value)
}

func (d RuntimeDomain) compoundContains(ctx context.Context, value RuntimeValue) (bool, error) {
	switch d.kind {
	case runtimeProductDomain:
		if value.expression.Kind != "app" || (value.expression.Opcode != "Tup" && value.expression.Opcode != "List") {
			return false, nil
		}
		children, err := runtimeChildren(value)
		if err != nil || len(children) != len(d.factors) {
			return false, err
		}
		for i, child := range children {
			contained, err := d.factors[i].Contains(ctx, child)
			if err != nil || !contained {
				return false, err
			}
		}
		return true, nil
	case runtimePowersetDomain:
		if value.expression.Kind != "app" {
			return false, nil
		}
		if value.expression.Opcode == "Map" || value.expression.Opcode == "Rec" {
			// Maps and records inherit an empty iterator despite being set-like.
			return true, nil
		}
		if value.expression.Opcode == "Tup" || value.expression.Opcode == "List" {
			children, err := runtimeChildren(value)
			if err != nil {
				return false, err
			}
			for _, child := range children {
				contained, err := d.base.Contains(ctx, child)
				if err != nil || !contained {
					return false, err
				}
			}
			return true, nil
		}
		if value.expression.Opcode != "Set" {
			return false, nil
		}
		domain, err := RuntimeDomainFromValue(value)
		if err != nil {
			return false, err
		}
		return d.ContainsDomain(ctx, domain)
	default:
		if value.expression.Kind != "app" || value.expression.Opcode != "Map" {
			return false, nil
		}
		entries, err := runtimeEntries(value)
		if err != nil {
			return false, err
		}
		keys := make([]RuntimeValue, len(entries))
		for i, entry := range entries {
			keys[i] = entry.Key
		}
		keySet, err := newRuntimeComposite("Set", keys)
		if err != nil {
			return false, err
		}
		keyDomain, err := RuntimeDomainFromValue(keySet)
		if err != nil {
			return false, err
		}
		equal, err := d.domain.Equal(ctx, keyDomain)
		if err != nil || !equal {
			return false, err
		}
		for _, entry := range entries {
			normal, err := runtimeNormalForm(entry.Value)
			if err != nil {
				return false, err
			}
			contained, err := d.rangeSet.Contains(ctx, normal)
			if err != nil || !contained {
				return false, err
			}
		}
		return true, nil
	}
}

func runtimeDomainValues(ctx context.Context, domain RuntimeDomain) ([]RuntimeValue, error) {
	values := []RuntimeValue{}
	err := domain.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		values = append(values, value)
		return true, nil
	})
	return values, err
}

func runtimeDomainMap(keys, values []RuntimeValue) (RuntimeValue, error) {
	entries := make([]runtimeReferenceEntry, len(keys))
	for i, key := range keys {
		normal, err := runtimeNormalForm(key)
		if err != nil {
			return RuntimeValue{}, err
		}
		entries[i] = runtimeReferenceEntry{Key: normal, Value: values[i]}
	}
	mapValue, err := runtimeReferenceMapFromEntries(entries)
	if err != nil {
		return RuntimeValue{}, err
	}
	return runtimeWithEntries("Map", mapValue)
}

func (d RuntimeDomain) powersetFromIndex(ctx context.Context, index Integer) (RuntimeValue, error) {
	bits, two := index.Big(), big.NewInt(2)
	values := []RuntimeValue{}
	err := d.base.ForEach(ctx, func(value RuntimeValue) (bool, error) {
		if new(big.Int).Rem(bits, two).Cmp(big.NewInt(1)) == 0 {
			values = append(values, value)
		}
		bits.Quo(bits, two)
		return true, nil
	})
	if err != nil {
		return RuntimeValue{}, err
	}
	return newRuntimeComposite("Set", values)
}

func (d RuntimeDomain) compoundForEach(ctx context.Context, yield func(RuntimeValue) (bool, error)) error {
	switch d.kind {
	case runtimePowersetDomain:
		size, err := d.Cardinality(ctx)
		if err != nil {
			if runtimeDomainIsDiagnostic(err) {
				return RuntimeDomainException{Name: "Error", Message: "Either state is Left"}
			}
			return err
		}
		one, last := big.NewInt(1), size.Big()
		for index := new(big.Int); index.Cmp(last) < 0; index.Add(index, one) {
			value, err := d.powersetFromIndex(ctx, IntegerFromBig(index))
			if err != nil {
				return err
			}
			more, err := yield(value)
			if err != nil || !more {
				return err
			}
		}
		return nil
	case runtimeProductDomain:
		arrays := make([][]RuntimeValue, len(d.factors))
		empty := false
		for i, factor := range d.factors {
			values, err := runtimeDomainValues(ctx, factor)
			if err != nil {
				return err
			}
			arrays[i] = values
			empty = empty || len(values) == 0
		}
		// The reference's zero-factor iterator is empty, despite cardinality 1.
		if empty || len(arrays) == 0 {
			return nil
		}
		indices := make([]int, len(arrays))
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			children := make([]RuntimeValue, len(arrays))
			for i := range arrays {
				children[i] = arrays[i][indices[i]]
			}
			value, err := newRuntimeComposite("Tup", children)
			if err != nil {
				return err
			}
			more, err := yield(value)
			if err != nil || !more {
				return err
			}
			advanced := false
			for i := range indices {
				indices[i]++
				if indices[i] < len(arrays[i]) {
					advanced = true
					break
				}
				indices[i] = 0
			}
			if !advanced {
				return nil
			}
		}
	default:
		keys, err := runtimeDomainValues(ctx, *d.domain)
		if err != nil {
			return err
		}
		rangeValues, err := runtimeDomainValues(ctx, *d.rangeSet)
		if err != nil {
			return err
		}
		if len(keys) == 0 {
			value, err := runtimeDomainMap(nil, nil)
			if err != nil {
				return err
			}
			_, err = yield(value)
			return err
		}
		if len(rangeValues) == 0 {
			return nil
		}
		// The source iterator uses Number indices/powers and least-significant
		// domain-key digits first, independently of BigInt cardinality.
		base := float64(len(rangeValues))
		limit := math.Pow(base, float64(len(keys)))
		for index := 0.0; index < limit; index++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			remaining := index
			values := make([]RuntimeValue, len(keys))
			for i := range keys {
				values[i] = rangeValues[int(math.Mod(remaining, base))]
				remaining = math.Floor(remaining / base)
			}
			value, err := runtimeDomainMap(keys, values)
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
}

func (d RuntimeDomain) compoundSubset(ctx context.Context, other RuntimeDomain) (bool, error) {
	switch d.kind {
	case runtimePowersetDomain:
		return d.base.Subset(ctx, *other.base)
	case runtimeMapSetDomain:
		equal, err := d.domain.Equal(ctx, *other.domain)
		if err != nil || !equal {
			return false, err
		}
		return d.rangeSet.Subset(ctx, *other.rangeSet)
	default:
		if len(d.factors) != len(other.factors) {
			return false, nil
		}
		for i, factor := range d.factors {
			subset, err := factor.Subset(ctx, other.factors[i])
			if err != nil || !subset {
				return false, err
			}
		}
		return true, nil
	}
}

func (d RuntimeDomain) compoundEqual(ctx context.Context, other RuntimeDomain) (bool, error) {
	switch d.kind {
	case runtimePowersetDomain:
		return d.base.Equal(ctx, *other.base)
	case runtimeMapSetDomain:
		equal, err := d.domain.Equal(ctx, *other.domain)
		if err != nil || !equal {
			return false, err
		}
		return d.rangeSet.Equal(ctx, *other.rangeSet)
	default:
		if len(d.factors) != len(other.factors) {
			return false, nil
		}
		for i, factor := range d.factors {
			equal, err := factor.Equal(ctx, other.factors[i])
			if err != nil || !equal {
				return false, err
			}
		}
		return true, nil
	}
}

type runtimeDomainPositions struct {
	values []Integer
	index  int
}

func (p *runtimeDomainPositions) next() (Integer, error) {
	index := p.index
	p.index++
	if index >= len(p.values) {
		return Integer{}, RuntimeDomainException{Name: "AssertionError", Message: "Internal error: too few positions. Report a bug."}
	}
	return p.values[index], nil
}

// PickWithPositions returns the selected value and number of iterator next
// calls, including a failed call when positions are exhausted. Nested picks
// share this cursor; caller positions are never modified.
func (d RuntimeDomain) PickWithPositions(ctx context.Context, positions []Integer) (RuntimeValue, int, error) {
	cursor := runtimeDomainPositions{values: positions}
	value, err := d.pickPosition(ctx, &cursor)
	return value, cursor.index, err
}

func (d RuntimeDomain) pickPosition(ctx context.Context, positions *runtimeDomainPositions) (RuntimeValue, error) {
	if err := d.validate(ctx); err != nil {
		return RuntimeValue{}, err
	}
	if d.kind == runtimeProductDomain {
		values := make([]RuntimeValue, len(d.factors))
		errs := []error{}
		for i, factor := range d.factors {
			value, err := factor.pickPosition(ctx, positions)
			if err != nil {
				if !runtimeDomainIsDiagnostic(err) {
					return RuntimeValue{}, err
				}
				errs = append(errs, err)
			}
			values[i] = value
		}
		if err := runtimeDomainMergedErrors(errs); err != nil {
			return RuntimeValue{}, err
		}
		return newRuntimeComposite("Tup", values)
	}
	if d.kind == runtimeMapSetDomain {
		return d.pickMap(ctx, positions)
	}
	position, err := positions.next()
	if err != nil {
		return RuntimeValue{}, err
	}
	if d.kind == runtimePowersetDomain {
		if _, err := d.Cardinality(ctx); err != nil {
			return RuntimeValue{}, err
		}
		return d.powersetFromIndex(ctx, position)
	}
	if d.kind == runtimeExplicitDomain {
		return PickRuntimePosition(ctx, d.explicit, []Integer{position})
	}
	n := position.Big()
	if d.kind == runtimeIntervalDomain {
		if d.last.Big().Cmp(d.first.Big()) < 0 {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Index out of bounds"}
		}
		n.Add(n, d.first.Big())
	} else if d.kind == runtimeNaturalDomain {
		n.Abs(n)
	}
	return NewRuntimeValue(Expr{Kind: "int", Int: IntegerFromBig(n)})
}

func (d RuntimeDomain) pickMap(ctx context.Context, positions *runtimeDomainPositions) (RuntimeValue, error) {
	domainSize, domainErr := d.domain.Cardinality(ctx)
	if domainErr != nil && !runtimeDomainIsDiagnostic(domainErr) {
		return RuntimeValue{}, domainErr
	}
	rangeSize, rangeErr := d.rangeSet.Cardinality(ctx)
	if rangeErr != nil && !runtimeDomainIsDiagnostic(rangeErr) {
		return RuntimeValue{}, rangeErr
	}
	if domainErr != nil {
		return RuntimeValue{}, domainErr
	}
	if domainSize.Big().Sign() == 0 {
		return runtimeDomainMap(nil, nil)
	}
	if rangeErr == nil && rangeSize.Big().Sign() == 0 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Empty set of maps"}
	}
	keys, values := []RuntimeValue{}, []RuntimeValue{}
	err := d.domain.ForEach(ctx, func(key RuntimeValue) (bool, error) {
		rangeDomain := *d.rangeSet
		if rangeDomain.kind == runtimeMapSetDomain {
			// The reference enumerates nested map ranges into a raw transient
			// Set, without the additional mkSet normalization boundary.
			entries := []runtimeReferenceEntry{}
			if err := rangeDomain.ForEach(ctx, func(value RuntimeValue) (bool, error) {
				entries = append(entries, runtimeReferenceEntry{Key: value, Value: value})
				return true, nil
			}); err != nil {
				return false, err
			}
			set, err := runtimeReferenceMapFromEntries(entries)
			if err != nil {
				return false, err
			}
			value, err := runtimeWithEntries("Set", set)
			if err != nil {
				return false, err
			}
			rangeDomain, err = RuntimeDomainFromValue(value)
			if err != nil {
				return false, err
			}
		}
		value, err := rangeDomain.pickPosition(ctx, positions)
		if err != nil {
			return false, err
		}
		keys, values = append(keys, key), append(values, value)
		return true, nil
	})
	if err != nil {
		return RuntimeValue{}, err
	}
	return runtimeDomainMap(keys, values)
}

func (d RuntimeDomain) compoundReferenceHashCode() (float64, error) {
	children := d.factors
	if d.kind == runtimePowersetDomain {
		return d.base.referenceHashCode()
	}
	if d.kind == runtimeMapSetDomain {
		children = []RuntimeDomain{*d.domain, *d.rangeSet}
	}
	var sum float64
	for _, child := range children {
		hash, err := child.referenceHashCode()
		if err != nil {
			return 0, err
		}
		sum += hash
	}
	return sum, nil
}

// referenceHash retains JavaScript Number accumulation, including values above
// int64, before applying Immutable.hash's ToUint32 and signed-small conversion.
func (d RuntimeDomain) referenceHash() (int32, error) {
	code, err := d.referenceHashCode()
	if err != nil {
		return 0, err
	}
	wrapped := math.Mod(code, 4294967296)
	if wrapped < 0 {
		wrapped += 4294967296
	}
	return runtimeReferenceSMI(uint32(wrapped)), nil
}
