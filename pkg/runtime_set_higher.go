package quint

import (
	"context"
	"fmt"
)

func setHigherBuiltinArity(op string) int {
	switch op {
	case "exists", "forall", "map", "filter", "mapBy":
		return 2
	case "fold":
		return 3
	}
	return 0
}

// EvalSetHigherBuiltin applies closures to explicit finite sets in retained
// entry order. Quantifiers apply every callback before testing their results;
// folds pass (accumulator, element) and preserve an empty fold's initial value.
func EvalSetHigherBuiltin(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	arity := setHigherBuiltinArity(op)
	if arity == 0 {
		return fail(fmt.Sprintf("Runtime does not support the built -in operator '%s'", op))
	}
	if len(args) != arity {
		return fail(fmt.Sprintf("%s requires %d arguments", op, arity))
	}
	for _, arg := range args {
		if !arg.Valid() {
			return fail("Invalid runtime value")
		}
	}
	if args[0].expression.Kind != "app" || args[0].expression.Opcode != "Set" {
		return fail(op + " requires a Set value")
	}
	entries, err := runtimeEntries(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	lambda := args[arity-1]
	// The reference resolves fold's callback only when reduction invokes it.
	if lambda.closure == nil && (op != "fold" || len(entries) > 0) {
		return fail("Expected a lambda value")
	}
	if op == "fold" {
		accumulator := args[1]
		for _, entry := range entries {
			accumulator, err = lambda.Apply(ctx, []RuntimeValue{accumulator, entry.Value})
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		return accumulator, nil
	}
	results := make([]runtimeReferenceEntry, 0, len(entries))
	selected := make([]RuntimeValue, 0, len(entries))
	for _, entry := range entries {
		value, err := lambda.Apply(ctx, []RuntimeValue{entry.Value})
		if err != nil {
			return RuntimeValue{}, err
		}
		switch op {
		case "filter":
			if value.expression.Kind != "bool" {
				return fail("Expected a Boolean value")
			}
			if value.expression.Bool {
				selected = append(selected, entry.Value)
			}
		case "mapBy":
			key, err := runtimeNormalForm(entry.Value)
			if err != nil {
				return RuntimeValue{}, err
			}
			results = append(results, runtimeReferenceEntry{Key: key, Value: value})
		default:
			results = append(results, runtimeReferenceEntry{Key: value, Value: value})
		}
	}
	if op == "filter" {
		return newRuntimeComposite("Set", selected)
	}
	// applyLambdaToSet and mapBy use transient Immutable constructors before
	// the operation's final normalization boundary. Preserve that traversal.
	resultSet, err := runtimeReferenceMapFromEntries(results)
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "mapBy" {
		return runtimeWithEntries("Map", resultSet)
	}
	if op == "map" {
		for _, entry := range resultSet.Entries() {
			selected = append(selected, entry.Value)
		}
		return newRuntimeComposite("Set", selected)
	}
	for _, entry := range resultSet.Entries() {
		if entry.Value.expression.Kind != "bool" {
			return fail("Expected a Boolean value")
		}
		if (op == "exists" && entry.Value.expression.Bool) || (op == "forall" && !entry.Value.expression.Bool) {
			return NewRuntimeValue(Expr{Kind: "bool", Bool: op == "exists"})
		}
	}
	return NewRuntimeValue(Expr{Kind: "bool", Bool: op == "forall"})
}
