package quint

import (
	"context"
	"fmt"
	"sort"

	"github.com/glycerine/insdict"
)

// RuntimeValueFromDomain retains an immutable compact domain without enumerating
// it. Expression and Equal expose its structural representation; their Context
// variants expose checked language display and extensional equality.
func RuntimeValueFromDomain(domain RuntimeDomain) (RuntimeValue, error) {
	return runtimeValueFromDomainContext(context.Background(), domain)
}

func runtimeValueFromDomainContext(ctx context.Context, domain RuntimeDomain) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if !domain.Valid() {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime domain"}
	}
	if domain.kind == runtimeExplicitDomain {
		return domain.explicit, nil
	}
	expression := runtimeDomainCompactExpression(domain)
	value := RuntimeValue{expression: expression, source: copyRuntimeExpression(expression), identity: normalizedValueKey(runtimeCompactIdentityExpression(expression)), allocation: &runtimeValueAllocation{}, domain: &domain, containsDomain: true, containsClosure: runtimeDomainContainsClosure(domain)}
	code, err := runtimeReferenceCodeContext(ctx, value)
	if err != nil {
		return RuntimeValue{}, err
	}
	value.referenceCode = &code
	return value, nil
}

func runtimeDomainCompactExpression(domain RuntimeDomain) Expr {
	integer := func(n Integer) Expr { return Expr{Kind: "int", Int: n} }
	child := func(d RuntimeDomain) Expr {
		if d.kind == runtimeExplicitDomain {
			return d.explicit.expression
		}
		return runtimeDomainCompactExpression(d)
	}
	switch domain.kind {
	case runtimeExplicitDomain:
		return domain.explicit.expression
	case runtimeIntervalDomain:
		return Expr{Kind: "app", Opcode: "to", Args: []Expr{integer(domain.first), integer(domain.last)}}
	case runtimeIntegerDomain, runtimeNaturalDomain:
		return Expr{Kind: "name", Name: domain.infiniteName()}
	case runtimeProductDomain:
		args := make([]Expr, len(domain.factors))
		for i, d := range domain.factors {
			args[i] = child(d)
		}
		return Expr{Kind: "app", Opcode: "tuples", Args: args}
	case runtimePowersetDomain:
		return Expr{Kind: "app", Opcode: "powerset", Args: []Expr{child(*domain.base)}}
	case runtimeMapSetDomain:
		return Expr{Kind: "app", Opcode: "setOfMaps", Args: []Expr{child(*domain.domain), child(*domain.rangeSet)}}
	}
	return Expr{}
}
func runtimeDomainContainsClosure(domain RuntimeDomain) bool {
	switch domain.kind {
	case runtimeExplicitDomain:
		return domain.explicit.containsClosure
	case runtimeProductDomain:
		for _, d := range domain.factors {
			if runtimeDomainContainsClosure(d) {
				return true
			}
		}
	case runtimePowersetDomain:
		return runtimeDomainContainsClosure(*domain.base)
	case runtimeMapSetDomain:
		return runtimeDomainContainsClosure(*domain.domain) || runtimeDomainContainsClosure(*domain.rangeSet)
	}
	return false
}
func runtimeCompactIdentityExpression(expression Expr) Expr {
	if expression.Kind == "name" && (expression.Name == "Int" || expression.Name == "Nat") {
		return Expr{Kind: "app", Opcode: "q::domain::" + expression.Name, Args: []Expr{}}
	}
	expression.Args = append([]Expr{}, expression.Args...)
	for i, child := range expression.Args {
		expression.Args[i] = runtimeCompactIdentityExpression(child)
	}
	return expression
}
func runtimeHasDomain(args []RuntimeValue) bool {
	for _, value := range args {
		if value.containsDomain {
			return true
		}
	}
	return false
}

// ExpressionContext returns the reference display, with cancellation and
// enumeration failures exposed to its caller. The returned expression is owned.
func (v RuntimeValue) ExpressionContext(ctx context.Context) (Expr, error) {
	return SerializeRuntimeValueContext(ctx, v, nil)
}

// SymbolicBuiltinArity describes compact domain constructors. A negative arity
// marks the variadic tuples constructor, which also accepts zero factors.
func SymbolicBuiltinArity(op string) int {
	switch op {
	case "to", "setOfMaps":
		return 2
	case "powerset":
		return 1
	case "tuples":
		return -1
	}
	return 0
}

// EvalSymbolicBuiltin constructs compact runtime domains without enumeration.
func EvalSymbolicBuiltin(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	arity := SymbolicBuiltinArity(op)
	if arity == 0 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: fmt.Sprintf("Runtime does not support the built -in operator '%s'", op)}
	}
	if arity > 0 && len(args) != arity {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: fmt.Sprintf("%s requires %d arguments", op, arity)}
	}
	for _, v := range args {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		if !v.Valid() {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
		}
	}
	if op == "to" {
		if args[0].expression.Kind != "int" || args[1].expression.Kind != "int" {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "to requires integer arguments"}
		}
		return runtimeValueFromDomainContext(ctx, NewRuntimeInterval(args[0].expression.Int, args[1].expression.Int))
	}
	domains := make([]RuntimeDomain, len(args))
	for i, v := range args {
		if err := ctx.Err(); err != nil {
			return RuntimeValue{}, err
		}
		d, err := RuntimeDomainFromValue(v)
		if err != nil {
			return RuntimeValue{}, err
		}
		domains[i] = d
	}
	var d RuntimeDomain
	var err error
	switch op {
	case "tuples":
		d, err = NewRuntimeProduct(domains)
	case "powerset":
		d, err = NewRuntimePowerset(domains[0])
	case "setOfMaps":
		d, err = NewRuntimeMapSet(domains[0], domains[1])
	}
	if err != nil {
		return RuntimeValue{}, err
	}
	return runtimeValueFromDomainContext(ctx, d)
}

// NewRuntimeCompositeContext constructs a collection from retained values. Set
// elements and map keys normalize at the same boundary as the reference runtime;
// other children retain compact domains until a checked operation requests them.
func NewRuntimeCompositeContext(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if !runtimeHasDomain(args) {
		return newRuntimeComposite(op, args)
	}
	for _, v := range args {
		if !v.Valid() {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
		}
	}
	switch op {
	case "List", "Tup":
		return runtimeAssembleCompact(ctx, op, append([]RuntimeValue{}, args...), runtimeReferenceMap{})
	case "variant":
		if len(args) != 2 || args[0].expression.Kind != "str" {
			return RuntimeValue{}, fmt.Errorf("variant requires a string label and value")
		}
		return runtimeAssembleCompact(ctx, op, append([]RuntimeValue{}, args...), runtimeReferenceMap{})
	case "Rec":
		if len(args)%2 != 0 {
			return RuntimeValue{}, fmt.Errorf("record requires field/value pairs")
		}
		fields := insdict.NewDict[string, [2]RuntimeValue]()
		for i := 0; i < len(args); i += 2 {
			if args[i].expression.Kind != "str" {
				return RuntimeValue{}, fmt.Errorf("record field must be a string")
			}
			fields.Set(args[i].expression.Str, [2]RuntimeValue{args[i], args[i+1]})
		}
		names := []string{}
		for name := range fields.All() {
			names = append(names, name)
		}
		sort.Slice(names, func(i, j int) bool { return utf16Less(names[i], names[j]) })
		children := make([]RuntimeValue, 0, len(names)*2)
		for _, name := range names {
			pair := fields.Get(name)
			children = append(children, pair[0], pair[1])
		}
		return runtimeAssembleCompact(ctx, op, children, runtimeReferenceMap{})
	case "Set":
		var m runtimeReferenceMap
		for _, v := range args {
			v, err := runtimeKeyNormalFormContext(ctx, v)
			if err != nil {
				return RuntimeValue{}, err
			}
			m, err = m.Set(v, v)
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		return runtimeWithEntriesContext(ctx, op, m)
	case "Map":
		entries := make([]runtimeReferenceEntry, 0, len(args))
		for _, pair := range args {
			if pair.expression.Kind != "app" || (pair.expression.Opcode != "Tup" && pair.expression.Opcode != "List") {
				return RuntimeValue{}, fmt.Errorf("map requires two-element tuple entries")
			}
			children, err := runtimeChildren(pair)
			if err != nil {
				return RuntimeValue{}, err
			}
			if len(children) != 2 {
				return RuntimeValue{}, fmt.Errorf("map requires two-element tuple entries")
			}
			key, err := runtimeKeyNormalFormContext(ctx, children[0])
			if err != nil {
				return RuntimeValue{}, err
			}
			entries = append(entries, runtimeReferenceEntry{Key: key, Value: children[1]})
		}
		m, err := runtimeReferenceMapFromEntries(entries)
		if err != nil {
			return RuntimeValue{}, err
		}
		return runtimeWithEntriesContext(ctx, op, m)
	}
	return RuntimeValue{}, fmt.Errorf("Cannot convert %s to a runtime value", op)
}
func runtimeAssembleCompact(ctx context.Context, op string, children []RuntimeValue, m runtimeReferenceMap) (RuntimeValue, error) {
	expression := Expr{Kind: "app", Opcode: op, Args: []Expr{}}
	source := Expr{Kind: "app", Opcode: op, Args: []Expr{}}
	containsClosure, containsDomain := false, false
	payload := &runtimePayload{children: children, entries: m}
	if op == "Set" || op == "Map" {
		for _, entry := range m.Entries() {
			containsClosure = containsClosure || entry.Key.containsClosure || entry.Value.containsClosure
			containsDomain = containsDomain || entry.Key.containsDomain || entry.Value.containsDomain
			if op == "Set" {
				expression.Args = append(expression.Args, entry.Value.expression)
				source.Args = append(source.Args, entry.Value.source)
			} else {
				expression.Args = append(expression.Args, Expr{Kind: "app", Opcode: "Tup", Args: []Expr{entry.Key.expression, entry.Value.expression}})
				source.Args = append(source.Args, Expr{Kind: "app", Opcode: "Tup", Args: []Expr{entry.Key.source, entry.Value.source}})
			}
		}
	} else {
		for _, child := range children {
			containsClosure = containsClosure || child.containsClosure
			containsDomain = containsDomain || child.containsDomain
			expression.Args = append(expression.Args, child.expression)
			source.Args = append(source.Args, child.source)
		}
	}
	payload.containsClosure = containsClosure
	value := RuntimeValue{expression: copyRuntimeExpression(expression), source: copyRuntimeExpression(source), identity: normalizedValueKey(runtimeCompactIdentityExpression(expression)), allocation: &runtimeValueAllocation{}, payload: payload, containsClosure: containsClosure, containsDomain: containsDomain}
	code, err := runtimeReferenceCodeContext(ctx, value)
	if err != nil {
		return RuntimeValue{}, err
	}
	value.referenceCode = &code
	return value, nil
}
func runtimeWithEntriesContext(ctx context.Context, op string, m runtimeReferenceMap) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	for _, entry := range m.Entries() {
		if entry.Key.containsDomain || entry.Value.containsDomain {
			return runtimeAssembleCompact(ctx, op, nil, m)
		}
	}
	return runtimeWithEntries(op, m)
}
func runtimeNormalFormContext(ctx context.Context, value RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if !value.Valid() {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
	}
	if value.domain != nil {
		return value.domain.Materialize(ctx)
	}
	if !value.containsDomain {
		return runtimeNormalForm(value)
	}
	switch value.expression.Opcode {
	case "variant":
		return value, nil
	case "Set":
		return runtimeWithEntriesContext(ctx, "Set", value.payload.entries)
	case "Map":
		entries := value.payload.entries.Entries()
		for i, entry := range entries {
			var err error
			entries[i].Key, err = runtimeKeyNormalFormContext(ctx, entry.Key)
			if err != nil {
				return RuntimeValue{}, err
			}
			entries[i].Value, err = runtimeNormalFormContext(ctx, entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		m, err := runtimeReferenceMapFromEntries(entries)
		if err != nil {
			return RuntimeValue{}, err
		}
		return runtimeWithEntriesContext(ctx, "Map", m)
	case "List", "Tup", "Rec":
		children := append([]RuntimeValue{}, value.payload.children...)
		for i, child := range children {
			var err error
			children[i], err = runtimeNormalFormContext(ctx, child)
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		return NewRuntimeCompositeContext(ctx, value.expression.Opcode, children)
	}
	return value, nil
}
func runtimeKeyNormalFormContext(ctx context.Context, value RuntimeValue) (RuntimeValue, error) {
	value, err := runtimeNormalFormContext(ctx, value)
	if err != nil {
		return RuntimeValue{}, err
	}
	if value.containsDomain {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Symbolic runtime values retained inside collection keys are not supported"}
	}
	return value, nil
}
func runtimeToSetContext(ctx context.Context, value RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if value.domain != nil {
		return value.domain.Materialize(ctx)
	}
	if value.Valid() && value.expression.Kind == "app" && value.expression.Opcode == "Set" {
		return value, nil
	}
	return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Expected a Set value"}
}

// Structural comparison never enumerates domains and preserves actual closure
// children, whose mock display expressions cannot define value equality.
func runtimeDomainStructuralEqual(left, right RuntimeDomain) bool {
	if left.kind != right.kind {
		return false
	}
	switch left.kind {
	case runtimeExplicitDomain:
		return left.explicit.Equal(right.explicit)
	case runtimeIntervalDomain:
		return left.first == right.first && left.last == right.last
	case runtimeIntegerDomain, runtimeNaturalDomain:
		return true
	case runtimeProductDomain:
		if len(left.factors) != len(right.factors) {
			return false
		}
		for i, factor := range left.factors {
			if !runtimeDomainStructuralEqual(factor, right.factors[i]) {
				return false
			}
		}
		return true
	case runtimePowersetDomain:
		return runtimeDomainStructuralEqual(*left.base, *right.base)
	case runtimeMapSetDomain:
		return runtimeDomainStructuralEqual(*left.domain, *right.domain) && runtimeDomainStructuralEqual(*left.rangeSet, *right.rangeSet)
	}
	return false
}
