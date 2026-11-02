package quint

import "crypto/sha256"

// RuntimeValue owns an immutable concrete language value. Its expression form is
// canonical for display; set/map identity does not depend on display collation.
// Closures and symbolic collections are introduced by the evaluator separately.
// The zero value is invalid, rather than an implicit language value.
type RuntimeValue struct {
	expression Expr
	identity   string
}

// NewRuntimeValue validates and copies a concrete expression, removing source IDs
// and canonicalizing records, sets and maps. The caller retains ownership of e.
func NewRuntimeValue(e Expr) (RuntimeValue, error) {
	normalized, err := NormalizeValue(e)
	if err != nil {
		return RuntimeValue{}, err
	}
	return RuntimeValue{expression: normalized, identity: normalizedValueKey(normalized)}, nil
}

func (v RuntimeValue) Valid() bool { return v.identity != "" }
func (v RuntimeValue) Equal(other RuntimeValue) bool {
	return v.Valid() && other.Valid() && v.identity == other.identity
}

// Hash is a deterministic structural hash for Go runtime collections. Its bytes
// need not equal either upstream runtime's implementation-specific hash. Equal
// values always hash equally, even when display strings have collation ties.
func (v RuntimeValue) Hash() [32]byte { return sha256.Sum256([]byte(v.identity)) }

// Expression returns a deep copy, so changing its argument slices cannot mutate
// the runtime value. Integer values are already immutable.
func (v RuntimeValue) Expression() Expr { return copyRuntimeExpression(v.expression) }
func (v RuntimeValue) String() string {
	if !v.Valid() {
		return "<invalid>"
	}
	return ExpressionString(v.expression)
}

func copyRuntimeExpression(e Expr) Expr {
	if e.Args != nil {
		children := make([]Expr, len(e.Args))
		for i, child := range e.Args {
			children[i] = copyRuntimeExpression(child)
		}
		e.Args = children
	}
	return e
}
