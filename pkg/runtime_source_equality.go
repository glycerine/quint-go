package quint

import "context"

// These are source runtime classes rather than public Go identity kinds. In
// particular List/Tup share one class; Int/Nat share another class whose kind
// test precedes the general set-like conversion fallback.
type runtimeSourceConstructor uint8

const (
	runtimeSourceInvalid runtimeSourceConstructor = iota
	runtimeSourceBool
	runtimeSourceInt
	runtimeSourceStr
	runtimeSourceSequence
	runtimeSourceRecord
	runtimeSourceVariant
	runtimeSourceSet
	runtimeSourceMap
	runtimeSourceInterval
	runtimeSourceProduct
	runtimeSourcePowerset
	runtimeSourceMapSet
	runtimeSourceInfinite
	runtimeSourceLambda
)

func runtimeSourceConstructorOf(value RuntimeValue) runtimeSourceConstructor {
	if value.domain != nil {
		switch value.domain.kind {
		case runtimeExplicitDomain:
			return runtimeSourceSet
		case runtimeIntervalDomain:
			return runtimeSourceInterval
		case runtimeProductDomain:
			return runtimeSourceProduct
		case runtimePowersetDomain:
			return runtimeSourcePowerset
		case runtimeMapSetDomain:
			return runtimeSourceMapSet
		case runtimeIntegerDomain, runtimeNaturalDomain:
			return runtimeSourceInfinite
		}
	}
	if value.closure != nil {
		return runtimeSourceLambda
	}
	switch value.expression.Kind {
	case "bool":
		return runtimeSourceBool
	case "int":
		return runtimeSourceInt
	case "str":
		return runtimeSourceStr
	case "lambda":
		return runtimeSourceLambda
	case "app":
		switch value.expression.Opcode {
		case "List", "Tup":
			return runtimeSourceSequence
		case "Rec":
			return runtimeSourceRecord
		case "variant":
			return runtimeSourceVariant
		case "Set":
			return runtimeSourceSet
		case "Map":
			return runtimeSourceMap
		}
	}
	return runtimeSourceInvalid
}
func runtimeSourceSetLike(constructor runtimeSourceConstructor) bool {
	switch constructor {
	case runtimeSourceSequence, runtimeSourceRecord, runtimeSourceSet, runtimeSourceMap, runtimeSourceInterval, runtimeSourceProduct, runtimeSourcePowerset, runtimeSourceMapSet, runtimeSourceInfinite:
		return true
	}
	return false
}

// The source equality fallback calls toSet from left to right. Lists/tuples
// normalize and deduplicate their iteration; records/maps inherit an empty
// iterator. This helper stays separate from typed builtin argument validation.
func runtimeSourceToSetContext(ctx context.Context, value RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if value.domain != nil {
		return value.domain.Materialize(ctx)
	}
	switch runtimeSourceConstructorOf(value) {
	case runtimeSourceSet:
		return value, nil
	case runtimeSourceSequence:
		children, err := runtimeChildren(value)
		if err != nil {
			return RuntimeValue{}, err
		}
		return NewRuntimeCompositeContext(ctx, "Set", children)
	case runtimeSourceRecord, runtimeSourceMap:
		return NewRuntimeCompositeContext(ctx, "Set", nil)
	}
	return RuntimeValue{}, RuntimeDomainException{Name: "Error", Message: "Expected a set-like value"}
}

// Same-class compact equality invokes equals on its retained factors. Routing
// those calls through the source helper preserves nested unlike-class toSet
// conversion and its error order, without changing the standalone domain API.
func runtimeSourceDomainEqualContext(ctx context.Context, left, right RuntimeDomain) (bool, error) {
	if err := left.validate(ctx); err != nil {
		return false, err
	}
	if err := right.validate(ctx); err != nil {
		return false, err
	}
	compare := func(a, b RuntimeDomain) (bool, error) {
		av, err := runtimeValueFromDomainContext(ctx, a)
		if err != nil {
			return false, err
		}
		bv, err := runtimeValueFromDomainContext(ctx, b)
		if err != nil {
			return false, err
		}
		return runtimeSourceEqualContext(ctx, av, bv)
	}
	if left.kind != right.kind {
		return compare(left, right)
	}
	switch left.kind {
	case runtimeExplicitDomain:
		return runtimeSourceEqualContext(ctx, left.explicit, right.explicit)
	case runtimeIntervalDomain:
		return left.first == right.first && left.last == right.last, nil
	case runtimeIntegerDomain, runtimeNaturalDomain:
		return true, nil
	case runtimeProductDomain:
		if len(left.factors) != len(right.factors) {
			return false, nil
		}
		for i, factor := range left.factors {
			equal, err := compare(factor, right.factors[i])
			if err != nil || !equal {
				return false, err
			}
		}
		return true, nil
	case runtimePowersetDomain:
		return compare(*left.base, *right.base)
	case runtimeMapSetDomain:
		equal, err := compare(*left.domain, *right.domain)
		if err != nil || !equal {
			return false, err
		}
		return compare(*left.rangeSet, *right.rangeSet)
	}
	return false, Diagnostic{Code: "QNT501", Message: "Invalid runtime domain"}
}
