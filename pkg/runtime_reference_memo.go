package quint

import (
	"context"
	"sync/atomic"
)

// Immutable.js caches a collection's hash after hashCode actually runs. The
// cache's presence can affect its equality shortcut when symbolic class hashes
// differ. Numeric referenceCode precomputation deliberately does not mark this
// source history. Atomic state keeps independent checked reads race-safe.
type runtimeReferenceMemo struct{ hashed atomic.Bool }

func runtimeReferenceMemoOf(value RuntimeValue) *runtimeReferenceMemo {
	if value.allocation == nil {
		return nil
	}
	if value.allocation.inheritedReferenceMemo != nil {
		return value.allocation.inheritedReferenceMemo
	}
	return &value.allocation.referenceMemo
}
func runtimeInheritReferenceMemo(value RuntimeValue, source RuntimeValue) RuntimeValue {
	if value.allocation != nil {
		value.allocation.inheritedReferenceMemo = runtimeReferenceMemoOf(source)
	}
	return value
}
func runtimeReferenceHashCached(value RuntimeValue) bool {
	memo := runtimeReferenceMemoOf(value)
	return memo != nil && memo.hashed.Load()
}

// runtimeMarkReferenceHashContext records only actual source hashCode calls.
// Callers must keep this distinct from computing an unused routing hash for a
// small array node. Published values and numeric hash caches remain immutable.
func runtimeMarkReferenceHashContext(ctx context.Context, value RuntimeValue) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if value.domain != nil {
		return runtimeMarkDomainReferenceHashContext(ctx, *value.domain)
	}
	if value.closure != nil || value.expression.Kind != "app" {
		return nil
	}
	if value.expression.Opcode == "variant" {
		children, err := runtimeChildren(value)
		if err != nil {
			return err
		}
		return runtimeMarkReferenceHashContext(ctx, children[1])
	}
	switch value.expression.Opcode {
	case "Set", "Map", "List", "Tup", "Rec":
	default:
		return nil
	}
	memo := runtimeReferenceMemoOf(value)
	if memo != nil && memo.hashed.Load() {
		return nil
	}
	if value.payload == nil {
		return nil
	}
	if value.expression.Opcode == "Set" || value.expression.Opcode == "Map" {
		for _, entry := range value.payload.entries.Entries() {
			if err := runtimeMarkReferenceHashContext(ctx, entry.Value); err != nil {
				return err
			}
			if value.expression.Opcode == "Map" {
				if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
					return err
				}
			}
		}
	} else {
		for _, child := range value.payload.children {
			if err := runtimeMarkReferenceHashContext(ctx, child); err != nil {
				return err
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if memo != nil {
		memo.hashed.Store(true)
	}
	return nil
}
func runtimeMarkDomainReferenceHashContext(ctx context.Context, domain RuntimeDomain) error {
	if err := domain.validate(ctx); err != nil {
		return err
	}
	switch domain.kind {
	case runtimeExplicitDomain:
		return runtimeMarkReferenceHashContext(ctx, domain.explicit)
	case runtimeProductDomain:
		for _, factor := range domain.factors {
			if err := runtimeMarkDomainReferenceHashContext(ctx, factor); err != nil {
				return err
			}
		}
	case runtimePowersetDomain:
		return runtimeMarkDomainReferenceHashContext(ctx, *domain.base)
	case runtimeMapSetDomain:
		if err := runtimeMarkDomainReferenceHashContext(ctx, *domain.domain); err != nil {
			return err
		}
		return runtimeMarkDomainReferenceHashContext(ctx, *domain.rangeSet)
	}
	return nil
}

func runtimeReferenceCachedHashesDiffer(ctx context.Context, left, right RuntimeValue) (bool, error) {
	if !runtimeReferenceHashCached(left) || !runtimeReferenceHashCached(right) {
		return false, nil
	}
	a, err := runtimeReferenceCodeContext(ctx, left)
	if err != nil {
		return false, err
	}
	b, err := runtimeReferenceCodeContext(ctx, right)
	return a != b, err
}

func runtimeKeyNeedsReferenceMemo(value RuntimeValue) bool {
	if value.domain != nil {
		return true
	}
	if value.expression.Kind != "app" || value.payload == nil {
		return false
	}
	if value.expression.Opcode == "variant" {
		return len(value.payload.children) == 2 && runtimeKeyNeedsReferenceMemo(value.payload.children[1])
	}
	switch value.expression.Opcode {
	case "Set", "Map", "List", "Tup", "Rec":
		return true
	}
	return false
}
func runtimeConstructorNeedsReferenceMemo(op string, args []RuntimeValue) bool {
	switch op {
	case "Set":
		for _, value := range args {
			if runtimeKeyNeedsReferenceMemo(value) {
				return true
			}
		}
	case "Map":
		for _, pair := range args {
			if pair.payload != nil && len(pair.payload.children) == 2 && runtimeKeyNeedsReferenceMemo(pair.payload.children[0]) {
				return true
			}
		}
	}
	return false
}
func runtimeMapOperationNeedsReferenceMemo(op string, args []RuntimeValue) bool {
	if len(args) > 0 && args[0].payload != nil && args[0].payload.referenceMemoKeys {
		return true
	}
	return len(args) > 1 && runtimeKeyNeedsReferenceMemo(args[1])
}
func runtimeSetOperationNeedsReferenceMemo(op string, args []RuntimeValue) bool {
	if op == "size" || op == "isFinite" {
		return false
	}
	for _, value := range args {
		if value.payload != nil && value.payload.referenceMemoKeys {
			return true
		}
	}
	if len(args) > 1 {
		if op == "contains" {
			return runtimeKeyNeedsReferenceMemo(args[1])
		}
		if op == "in" {
			return runtimeKeyNeedsReferenceMemo(args[0])
		}
	}
	return false
}
func runtimeSequenceInheritReferenceMemo(op string, args []RuntimeValue, result RuntimeValue) RuntimeValue {
	if len(args) == 0 || args[0].payload == nil || result.allocation == nil {
		return result
	}
	source := args[0]
	children := source.payload.children
	switch op {
	case "replaceAt":
		if len(args) == 3 && args[1].expression.Kind == "int" {
			index, _ := args[1].expression.Int.Big().Float64()
			if index >= 0 && index < float64(len(children)) && sameRuntimeObject(children[int(index)], args[2]) {
				return runtimeInheritReferenceMemo(result, source)
			}
		}
	case "slice":
		if len(args) == 3 && args[1].expression.Kind == "int" && args[2].expression.Kind == "int" {
			end, _ := args[2].expression.Int.Big().Float64()
			if args[1].expression.Int.Big().Sign() == 0 && end == float64(len(children)) {
				return runtimeInheritReferenceMemo(result, source)
			}
		}
	case "concat":
		if len(args) == 2 && args[1].payload != nil {
			if len(args[1].payload.children) == 0 {
				return runtimeInheritReferenceMemo(result, source)
			}
			if len(children) == 0 {
				return runtimeInheritReferenceMemo(result, args[1])
			}
		}
	}
	return result
}
