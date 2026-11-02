package quint

import (
	"context"
	"crypto/sha256"
	"fmt"
)

// RuntimeValue owns a concrete language value or a lexical closure. Its expression form is
// canonical for display; set/map identity does not depend on display collation.
// Closures are created by the evaluator; symbolic collections remain separate.
// The zero value is invalid, rather than an implicit language value.
type RuntimeValue struct {
	expression Expr
	source     Expr
	identity   string
	closure    *runtimeClosure
}

// NewRuntimeValue validates and copies a concrete expression, removing source IDs
// and canonicalizing records, sets and maps. The caller retains ownership of e.
func NewRuntimeValue(e Expr) (RuntimeValue, error) {
	normalized, err := NormalizeValue(e)
	if err != nil {
		return RuntimeValue{}, err
	}
	return RuntimeValue{expression: normalized, source: copyRuntimeExpression(e), identity: normalizedValueKey(normalized)}, nil
}

func (v RuntimeValue) Valid() bool { return v.identity != "" || v.closure != nil }
func (v RuntimeValue) Equal(other RuntimeValue) bool {
	return v.Valid() && other.Valid() && v.closure == nil && other.closure == nil && v.identity == other.identity
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
	if e.Body != nil {
		body := copyRuntimeExpression(*e.Body)
		e.Body = &body
	}
	if e.Params != nil {
		e.Params = append([]Parameter{}, e.Params...)
	}
	if e.Opdef != nil {
		definition := *e.Opdef
		if definition.Expr != nil {
			value := copyRuntimeExpression(*definition.Expr)
			definition.Expr = &value
		}
		e.Opdef = &definition
	}
	if e.Args != nil {
		children := make([]Expr, len(e.Args))
		for i, child := range e.Args {
			children[i] = copyRuntimeExpression(child)
		}
		e.Args = children
	}
	return e
}

type runtimeClosure struct {
	apply func(context.Context, []RuntimeValue) (RuntimeValue, error)
}

// Apply invokes a runtime closure. Closure evaluations share their owning
// evaluator's scope machinery and must not be run concurrently.
func (v RuntimeValue) Apply(ctx context.Context, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if v.closure == nil {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Expected a lambda value"}
	}
	for _, arg := range args {
		if !arg.Valid() {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime argument"}
		}
	}
	return v.closure.apply(ctx, args)
}

func newRuntimeClosure(arity int, apply func(context.Context, []RuntimeValue) (RuntimeValue, error)) RuntimeValue {
	body := Expr{Kind: "str", Str: fmt.Sprintf("lambda_%d_params", arity)}
	expression := Expr{Kind: "lambda", Qualifier: Def, Params: []Parameter{}, Body: &body}
	for i := 0; i < arity; i++ {
		expression.Params = append(expression.Params, Parameter{Name: fmt.Sprintf("_a%d", i)})
	}
	return RuntimeValue{expression: expression, source: expression, closure: &runtimeClosure{apply: apply}}
}
