package quint

import (
	"context"
	"crypto/sha256"
	"fmt"
)

// Allocation identity is distinct from structural equality. It supports the
// reference collection no-op rule for reusing the same stored value object.
// The non-zero-sized token prevents Go from coalescing allocation addresses.
type runtimeValueAllocation struct{ marker byte }

// RuntimeValue owns a language value, a lexical closure, or a collection retaining
// such values, including compact symbolic domains. Its context-free expression
// is a structural view; ExpressionContext supplies checked language display.
// Set/map identity does not depend on display collation. Closures are created
// by the evaluator.
// The zero value is invalid, rather than an implicit language value.
type RuntimeValue struct {
	expression Expr
	source     Expr
	identity   string
	closure    *runtimeClosure
	allocation *runtimeValueAllocation
	payload    *runtimePayload
	// containsClosure separates executable payload identity from the mock lambda
	// expressions that the reference emits solely for display.
	containsClosure bool
	// Compact domains and their containing values keep executable payloads
	// separately from this context-free structural view. Checked operations
	// expose extensional semantics and errors without eager materialization.
	domain         *RuntimeDomain
	containsDomain bool
	referenceCode  *float64
}

// NewRuntimeValue validates and copies a concrete expression, removing source IDs
// and canonicalizing records, sets and maps. The caller retains ownership of e.
func NewRuntimeValue(e Expr) (RuntimeValue, error) {
	if e.Kind == "app" {
		switch e.Opcode {
		case "List", "Tup", "Rec", "Set", "Map", "variant":
			children := make([]RuntimeValue, len(e.Args))
			for i, child := range e.Args {
				value, err := NewRuntimeValue(child)
				if err != nil {
					return RuntimeValue{}, err
				}
				children[i] = value
			}
			return newRuntimeComposite(e.Opcode, children)
		}
	}
	return runtimeAssemble(e, nil)
}

func (v RuntimeValue) Valid() bool { return v.identity != "" || v.closure != nil }

// Equal compares concrete values or compact structural representations. Use
// EqualContext for extensional language equality involving symbolic domains.
func (v RuntimeValue) Equal(other RuntimeValue) bool {
	if !v.Valid() || !other.Valid() || v.closure != nil || other.closure != nil {
		return false
	}
	if v.domain != nil || other.domain != nil {
		return v.domain != nil && other.domain != nil && runtimeDomainStructuralEqual(*v.domain, *other.domain)
	}
	if !v.containsClosure && !other.containsClosure {
		return v.identity == other.identity
	}
	if v.expression.Kind != other.expression.Kind || v.expression.Opcode != other.expression.Opcode {
		return false
	}
	if v.payload == nil || other.payload == nil {
		return v.identity == other.identity
	}
	switch v.expression.Opcode {
	case "variant":
		left, right := v.payload.children, other.payload.children
		// Variant.equals calls its payload's equals directly, without Immutable's
		// object-identity short circuit. In particular, variant(f) != variant(f).
		return len(left) == 2 && len(right) == 2 && left[0].Equal(right[0]) && left[1].Equal(right[1])
	case "List", "Tup", "Rec":
		left, right := v.payload.children, other.payload.children
		if len(left) != len(right) {
			return false
		}
		for i, child := range left {
			if !runtimeCollectionEqual(child, right[i]) {
				return false
			}
		}
		return true
	case "Set", "Map":
		left, right := v.payload.entries, other.payload.entries
		if left.Len() != right.Len() {
			return false
		}
		for _, entry := range left.Entries() {
			value, found, err := right.Lookup(entry.Key)
			if err != nil || !found || !runtimeCollectionEqual(entry.Value, value) {
				return false
			}
		}
		return true
	}
	return false
}

// Immutable collections compare object identity before invoking equals. Lambdas
// cannot compare directly, but a collection can retain and recognize one object.
func runtimeCollectionEqual(left, right RuntimeValue) bool {
	return sameRuntimeObject(left, right) || left.Equal(right)
}

// Hash is a deterministic structural hash for Go runtime collections. Its bytes
// need not equal either upstream runtime's implementation-specific hash. Equal
// values always hash equally, even when display strings have collation ties.
// Mock lambda expressions may collide; executable closure identity is compared
// separately when collections contain closures.
// Compact domains use structural identity; HashContext supplies an extensional
// hash and exposes enumeration failures and cancellation.
func (v RuntimeValue) Hash() [32]byte { return sha256.Sum256([]byte(v.identity)) }

// Expression returns a deep copy, so changing its argument slices cannot mutate
// the runtime value. Integer values are already immutable. Compact domains keep
// constructor IR here; ExpressionContext supplies checked language display.
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
	return RuntimeValue{expression: expression, source: expression, closure: &runtimeClosure{apply: apply}, allocation: &runtimeValueAllocation{}, containsClosure: true}
}

// sameRuntimeObject follows reference object identity without changing language
// structural equality. Copies of one RuntimeValue keep the same token.
func sameRuntimeObject(left, right RuntimeValue) bool {
	return (left.allocation != nil && left.allocation == right.allocation) ||
		(left.closure != nil && left.closure == right.closure)
}
