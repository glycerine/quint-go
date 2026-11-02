package quint

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
)

// EqualContext compares language values extensionally, including compact
// domains retained inside collections. Domain comparisons can fail or be
// canceled; Equal provides only the context-free structural view of these
// values. Concrete values keep the existing equality and closure rules.
func (v RuntimeValue) EqualContext(ctx context.Context, other RuntimeValue) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if !v.Valid() || !other.Valid() {
		return false, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
	}
	if !v.containsDomain && !other.containsDomain {
		return v.Equal(other), nil
	}
	if v.domain != nil || other.domain != nil {
		isDomain := func(value RuntimeValue) bool {
			return value.domain != nil || (value.expression.Kind == "app" && value.expression.Opcode == "Set")
		}
		if !isDomain(v) || !isDomain(other) {
			return false, nil
		}
		left, err := RuntimeDomainFromValue(v)
		if err != nil {
			return false, err
		}
		right, err := RuntimeDomainFromValue(other)
		if err != nil {
			return false, err
		}
		return left.Equal(ctx, right)
	}
	if v.closure != nil || other.closure != nil || v.expression.Kind != other.expression.Kind || v.expression.Opcode != other.expression.Opcode {
		return false, nil
	}
	if v.payload == nil || other.payload == nil {
		return false, fmt.Errorf("checked equality requires retained collection payloads")
	}
	switch v.expression.Opcode {
	case "variant", "List", "Tup", "Rec":
		left, right := v.payload.children, other.payload.children
		if len(left) != len(right) {
			return false, nil
		}
		for i, child := range left {
			var equal bool
			var err error
			if v.expression.Opcode == "variant" {
				// Variant.equals invokes payload.equals directly. A closure
				// payload does not acquire collection object identity here.
				equal, err = child.EqualContext(ctx, right[i])
			} else {
				equal, err = runtimeCollectionEqualContext(ctx, child, right[i])
			}
			if err != nil || !equal {
				return false, err
			}
		}
		return true, nil
	case "Set", "Map":
		left, right := v.payload.entries, other.payload.entries
		if left.Len() != right.Len() {
			return false, nil
		}
		for _, entry := range left.Entries() {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			// Checked constructors normalize all keys before insertion, so
			// reference lookup retains its concrete key and traversal rules.
			value, found, err := right.Lookup(entry.Key)
			if err != nil || !found {
				return false, err
			}
			equal, err := runtimeCollectionEqualContext(ctx, entry.Value, value)
			if err != nil || !equal {
				return false, err
			}
		}
		return true, nil
	default:
		return false, fmt.Errorf("checked equality does not support %s", v.expression.Opcode)
	}
}

func runtimeCollectionEqualContext(ctx context.Context, left, right RuntimeValue) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if sameRuntimeObject(left, right) {
		return true, nil
	}
	return left.EqualContext(ctx, right)
}

// HashContext returns an extensional structural hash. EqualContext values have
// the same hash, including finite domains and their explicit Set forms. Hashing
// finite domains may enumerate them and returns cancellation or domain errors.
// Infinite Int and Nat domains use distinct private identity markers.
func (v RuntimeValue) HashContext(ctx context.Context) ([32]byte, error) {
	if err := ctx.Err(); err != nil {
		return [32]byte{}, err
	}
	if !v.Valid() {
		return [32]byte{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
	}
	if !v.containsDomain {
		return v.Hash(), nil
	}
	expression, err := runtimeIdentityExpressionContext(ctx, v)
	if err != nil {
		return [32]byte{}, err
	}
	hash := sha256.Sum256([]byte(normalizedValueKey(expression)))
	if err := ctx.Err(); err != nil {
		return [32]byte{}, err
	}
	return hash, nil
}

func runtimeIdentityExpressionContext(ctx context.Context, value RuntimeValue) (Expr, error) {
	if err := ctx.Err(); err != nil {
		return Expr{}, err
	}
	if !value.Valid() {
		return Expr{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
	}
	if value.domain != nil {
		if err := value.domain.validate(ctx); err != nil {
			return Expr{}, err
		}
		if value.domain.infinite() {
			// normalizedValueKey intentionally ignores names, so a unique
			// opcode, rather than a name expression, carries this identity.
			return Expr{Kind: "app", Opcode: "q::domain::" + value.domain.infiniteName(), Args: []Expr{}}, nil
		}
		normalized, err := value.domain.Materialize(ctx)
		if err != nil {
			return Expr{}, err
		}
		return runtimeIdentityExpressionContext(ctx, normalized)
	}
	if !value.containsDomain {
		return value.expression, nil
	}
	if value.payload == nil {
		return Expr{}, fmt.Errorf("checked hashing requires retained collection payloads")
	}
	expression := value.expression
	expression.Args = []Expr{}
	switch expression.Opcode {
	case "List", "Tup", "Rec", "variant":
		for _, child := range value.payload.children {
			normalized, err := runtimeIdentityExpressionContext(ctx, child)
			if err != nil {
				return Expr{}, err
			}
			expression.Args = append(expression.Args, normalized)
		}
	case "Set", "Map":
		for _, entry := range value.payload.entries.Entries() {
			normalized, err := runtimeIdentityExpressionContext(ctx, entry.Value)
			if err != nil {
				return Expr{}, err
			}
			if expression.Opcode == "Map" {
				key, err := runtimeIdentityExpressionContext(ctx, entry.Key)
				if err != nil {
					return Expr{}, err
				}
				normalized = Expr{Kind: "app", Opcode: "Tup", Args: []Expr{key, normalized}}
			}
			expression.Args = append(expression.Args, normalized)
		}
	default:
		return Expr{}, fmt.Errorf("checked hashing does not support %s", expression.Opcode)
	}
	return expression, nil
}

// runtimeReferenceHashContext implements Immutable.hash without expanding
// compact domains. It is separate from the extensional public HashContext.
func runtimeReferenceHashContext(ctx context.Context, value RuntimeValue) (int32, error) {
	code, err := runtimeReferenceCodeContext(ctx, value)
	return runtimeReferenceNumberSMI(code), err
}

// JavaScript's bitwise operations first apply ToUint32 to a Number, including
// Number rounding above int64 and zero conversion for infinities and NaN.
func runtimeReferenceNumberSMI(code float64) int32 {
	if math.IsInf(code, 0) || math.IsNaN(code) {
		return 0
	}
	wrapped := math.Mod(math.Trunc(code), 4294967296)
	if wrapped < 0 {
		wrapped += 4294967296
	}
	return runtimeReferenceSMI(uint32(wrapped))
}

// runtimeReferenceCodeContext returns the class's raw JavaScript Number
// hashCode. Retained payloads are hashed directly, preserving compact domain
// class hashes, closure identity behavior, and Number addition rounding.
func runtimeReferenceCodeContext(ctx context.Context, value RuntimeValue) (float64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !value.Valid() {
		return 0, fmt.Errorf("reference hashing requires a valid runtime value")
	}
	if value.referenceCode != nil {
		return *value.referenceCode, nil
	}
	if value.domain != nil {
		return runtimeDomainReferenceCodeContext(ctx, *value.domain)
	}
	if !value.containsDomain {
		code, err := runtimeReferenceHashCode(value.expression)
		return float64(code), err
	}
	if value.payload == nil {
		return 0, fmt.Errorf("reference hashing requires retained collection payloads")
	}
	if value.expression.Opcode == "variant" {
		children := value.payload.children
		if len(children) != 2 || children[0].expression.Kind != "str" {
			return 0, fmt.Errorf("invalid variant for reference hashing")
		}
		code, err := runtimeReferenceCodeContext(ctx, children[1])
		return float64(runtimeReferenceNumberSMI(code)) + code, err
	}
	var h uint32
	size := 0
	switch value.expression.Opcode {
	case "List", "Tup", "Rec":
		h = 1
		children := value.payload.children
		for i := 0; i < len(children); i++ {
			child, err := runtimeReferenceHashContext(ctx, children[i])
			if err != nil {
				return 0, err
			}
			if value.expression.Opcode == "Rec" {
				i++
				if i >= len(children) {
					return 0, fmt.Errorf("invalid record for reference hashing")
				}
				valueHash, err := runtimeReferenceHashContext(ctx, children[i])
				if err != nil {
					return 0, err
				}
				child = runtimeReferenceHashMerge(valueHash, child)
			}
			h = 31*h + uint32(child)
			size++
		}
	case "Set", "Map":
		for _, entry := range value.payload.entries.Entries() {
			child, err := runtimeReferenceHashContext(ctx, entry.Value)
			if err != nil {
				return 0, err
			}
			if value.expression.Opcode == "Map" {
				key, err := runtimeReferenceHashContext(ctx, entry.Key)
				if err != nil {
					return 0, err
				}
				child = runtimeReferenceHashMerge(child, key)
			}
			h += uint32(child)
			size++
		}
	default:
		return 0, fmt.Errorf("reference hashing does not support %s", value.expression.Opcode)
	}
	return float64(runtimeReferenceMurmur(uint32(size), h)), nil
}

func runtimeDomainReferenceCodeContext(ctx context.Context, domain RuntimeDomain) (float64, error) {
	if err := domain.validate(ctx); err != nil {
		return 0, err
	}
	switch domain.kind {
	case runtimeExplicitDomain:
		return runtimeReferenceCodeContext(ctx, domain.explicit)
	case runtimePowersetDomain:
		return runtimeDomainReferenceCodeContext(ctx, *domain.base)
	case runtimeProductDomain, runtimeMapSetDomain:
		children := domain.factors
		if domain.kind == runtimeMapSetDomain {
			children = []RuntimeDomain{*domain.domain, *domain.rangeSet}
		}
		var sum float64
		for _, child := range children {
			code, err := runtimeDomainReferenceCodeContext(ctx, child)
			if err != nil {
				return 0, err
			}
			sum += code
		}
		return sum, nil
	default:
		return domain.referenceHashCode()
	}
}
