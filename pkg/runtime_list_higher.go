package quint

import (
	"context"
	"fmt"
)

func listHigherBuiltinArity(op string) int {
	switch op {
	case "select":
		return 2
	case "foldl", "foldr":
		return 3
	}
	return 0
}

// EvalListHigherBuiltin applies closures in sequence order. foldl passes
// (accumulator, element); foldr traverses right-to-left and passes
// (element, accumulator). Empty folds return the initial value unchanged.
func EvalListHigherBuiltin(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	arity := listHigherBuiltinArity(op)
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
	if args[0].expression.Kind != "app" || args[0].expression.Opcode != "List" {
		return fail(op + " requires a List value")
	}
	lambda := args[arity-1]
	elements, err := runtimeChildren(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	// The reference resolves a select predicate before traversal, while fold
	// callbacks are resolved only when an element actually invokes them.
	if lambda.closure == nil && (op == "select" || len(elements) > 0) {
		return fail("Expected a lambda value")
	}
	if op == "select" {
		selected := []RuntimeValue{}
		for _, element := range elements {
			predicate, err := lambda.Apply(ctx, []RuntimeValue{element})
			if err != nil {
				return RuntimeValue{}, err
			}
			if predicate.expression.Kind != "bool" {
				return fail("Expected a Boolean value")
			}
			if predicate.expression.Bool {
				selected = append(selected, element)
			}
		}
		return newRuntimeComposite("List", selected)
	}
	accumulator := args[1]
	for i := 0; i < len(elements); i++ {
		index := i
		if op == "foldr" {
			index = len(elements) - 1 - i
		}
		value := elements[index]
		parameters := []RuntimeValue{accumulator, value}
		if op == "foldr" {
			parameters[0], parameters[1] = value, accumulator
		}
		accumulator, err = lambda.Apply(ctx, parameters)
		if err != nil {
			return RuntimeValue{}, err
		}
	}
	return accumulator, nil
}
