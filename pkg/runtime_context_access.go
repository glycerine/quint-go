package quint

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// EvalMapBuiltinContext retains compact symbolic map values while normalizing
// keys with cancellation and explicit errors for non-enumerable domains.
func EvalMapBuiltinContext(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if !runtimeHasDomain(args) && !runtimeMapOperationNeedsReferenceMemo(op, args) {
		value, err := EvalMapBuiltin(op, args)
		if err == nil && (op == "set" || op == "put") && value.payload != nil && args[0].payload != nil && value.payload.entries.root == args[0].payload.entries.root {
			value = runtimeInheritReferenceMemo(value, args[0])
		}
		return value, err
	}
	fail := func(code, message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: code, Message: message}
	}
	arity := mapBuiltinArity(op)
	if arity == 0 {
		return fail("QNT501", fmt.Sprintf("Runtime does not support the built -in operator '%s'", op))
	}
	if len(args) != arity {
		return fail("QNT501", fmt.Sprintf("%s requires %d arguments", op, arity))
	}
	for _, arg := range args {
		if !arg.Valid() {
			return fail("QNT501", "Invalid runtime value")
		}
	}
	if args[0].expression.Kind != "app" || args[0].expression.Opcode != "Map" {
		return fail("QNT501", op+" requires a Map value")
	}
	entries, err := runtimeMap(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "keys" {
		keys := make([]RuntimeValue, 0, entries.Len())
		for _, entry := range entries.Entries() {
			keys = append(keys, entry.Key)
		}
		return NewRuntimeCompositeContext(ctx, "Set", keys)
	}
	key, err := runtimeKeyNormalFormContext(ctx, args[1])
	if err != nil {
		return RuntimeValue{}, err
	}
	value, found, err := entries.LookupContext(ctx, key)
	if err != nil {
		return RuntimeValue{}, err
	}
	if op == "get" {
		if found {
			return value, nil
		}
		// The raw Immutable.Map.toMap rebuild in the reference's diagnostic
		// path affects collision traversal, unlike RuntimeValue.toMap.
		diagnosticEntries, err := runtimeReferenceMapFromEntriesContext(ctx, entries.Entries())
		if err != nil {
			return RuntimeValue{}, err
		}
		objects := &RuntimeObjectHashState{}
		requested, err := SerializeRuntimeValueContext(ctx, key, objects)
		if err != nil {
			return RuntimeValue{}, err
		}
		names := make([]string, 0, entries.Len())
		for _, entry := range diagnosticEntries.Entries() {
			expression, err := SerializeRuntimeValueContext(ctx, entry.Key, objects)
			if err != nil {
				return RuntimeValue{}, err
			}
			names = append(names, ExpressionString(expression))
		}
		return fail("QNT507", fmt.Sprintf("Called 'get' with a non-existing key. Key is %s. Map has keys: %s", ExpressionString(requested), strings.Join(names, ", ")))
	}
	if op == "set" && !found {
		return fail("QNT507", "Called 'set' with a non-existing key")
	}
	updated, err := entries.SetContext(ctx, key, args[2])
	if err != nil {
		return RuntimeValue{}, err
	}
	result, err := runtimeWithEntriesContext(ctx, "Map", updated)
	if err == nil && updated.root == entries.root {
		result = runtimeInheritReferenceMemo(result, args[0])
	}
	return result, err
}

// EvalMapSetByContext applies a callback to an existing retained value. Missing
// keys fail before invoking the callback, including compact symbolic keys.
func EvalMapSetByContext(ctx context.Context, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if len(args) != 3 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "setBy requires 3 arguments"}
	}
	current, err := EvalMapBuiltinContext(ctx, "get", args[:2])
	if err != nil {
		var d Diagnostic
		if errors.As(err, &d) && d.Code == "QNT507" {
			return RuntimeValue{}, Diagnostic{Code: "QNT507", Message: "Called 'setBy' with a non- existing key [object Object]"}
		}
		return RuntimeValue{}, err
	}
	updated, err := args[2].Apply(ctx, []RuntimeValue{current})
	if err != nil {
		return RuntimeValue{}, err
	}
	return EvalMapBuiltinContext(ctx, "set", []RuntimeValue{args[0], args[1], updated})
}

// EvalSequenceBuiltinContext accesses immutable retained children and builds
// updated collections without enumerating compact domains stored inside them.
// indices returns a compact inclusive interval, including for concrete lists.
func EvalSequenceBuiltinContext(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if op != "indices" && !runtimeHasDomain(args) {
		value, err := EvalSequenceBuiltin(op, args)
		if err == nil {
			value = runtimeSequenceInheritReferenceMemo(op, args, value)
		}
		return value, err
	}
	fail := func(code, message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: code, Message: message}
	}
	arity := sequenceBuiltinArity(op)
	if arity == 0 {
		return fail("QNT501", fmt.Sprintf("Runtime does not support the built -in operator '%s'", op))
	}
	if len(args) != arity {
		return fail("QNT501", fmt.Sprintf("%s requires %d arguments", op, arity))
	}
	for _, arg := range args {
		if !arg.Valid() {
			return fail("QNT501", "Invalid runtime value")
		}
	}
	record := op == "field" || op == "fieldNames" || op == "with"
	expected := "List"
	if record {
		expected = "Rec"
	}
	if op == "item" {
		expected = "Tup"
	}
	if args[0].expression.Kind != "app" || args[0].expression.Opcode != expected {
		return fail("QNT501", op+" requires a "+expected+" value")
	}
	elems, err := runtimeChildren(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	makeValue := func(constructor string, values []RuntimeValue) (RuntimeValue, error) {
		value, err := NewRuntimeCompositeContext(ctx, constructor, values)
		if err == nil {
			value = runtimeSequenceInheritReferenceMemo(op, args, value)
		}
		return value, err
	}
	if record {
		if op == "fieldNames" {
			names := []RuntimeValue{}
			for i := 0; i < len(elems); i += 2 {
				names = append(names, elems[i])
			}
			return makeValue("Set", names)
		}
		if args[1].expression.Kind != "str" {
			return fail("QNT501", op+" requires a string field")
		}
		name := args[1].expression.Str
		for i := len(elems) - 2; i >= 0; i -= 2 {
			if elems[i].expression.Str == name {
				if op == "field" {
					return elems[i+1], nil
				}
				elems[i+1] = args[2]
				return makeValue("Rec", elems)
			}
		}
		if op == "field" {
			return fail("QNT501", "Accessing a missing record field "+name)
		}
		return fail("QNT501", "Called 'with' with a non-existent field "+name)
	}
	switch op {
	case "indices":
		last, _ := ParseInteger(fmt.Sprint(len(elems) - 1))
		return RuntimeValueFromDomain(NewRuntimeInterval(Integer{}, last))
	case "length":
		n, _ := ParseInteger(fmt.Sprint(len(elems)))
		return NewRuntimeValue(Expr{Kind: "int", Int: n})
	case "head", "tail":
		if len(elems) == 0 {
			return fail("QNT505", fmt.Sprintf("Called '%s' on an empty list", op))
		}
		if op == "head" {
			return elems[0], nil
		}
		return makeValue("List", elems[1:])
	case "append":
		return makeValue("List", append(elems, args[1]))
	case "concat":
		if args[1].expression.Kind != "app" || args[1].expression.Opcode != "List" {
			return fail("QNT501", "concat requires List values")
		}
		other, err := runtimeChildren(args[1])
		if err != nil {
			return RuntimeValue{}, err
		}
		return makeValue("List", append(elems, other...))
	}
	if args[1].expression.Kind != "int" {
		return fail("QNT501", op+" requires integer indices")
	}
	index, _ := args[1].expression.Int.Big().Float64()
	if op == "item" {
		index--
	}
	valid := index >= 0 && index < float64(len(elems))
	switch op {
	case "item", "nth":
		if !valid {
			return fail("QNT510", "Out of bounds, nth("+runtimeNumberString(index)+")")
		}
		return elems[int(index)], nil
	case "replaceAt":
		if !valid {
			return fail("QNT510", "Out of bounds, replaceAt("+runtimeNumberString(index)+")")
		}
		elems[int(index)] = args[2]
		return makeValue("List", elems)
	case "slice":
		if args[2].expression.Kind != "int" {
			return fail("QNT501", "slice requires integer indices")
		}
		end, _ := args[2].expression.Int.Big().Float64()
		if index < 0 || index > end || end > float64(len(elems)) {
			return fail("QNT506", fmt.Sprintf("slice(..., %s, %s) applied to a list of size %d", runtimeNumberString(index), runtimeNumberString(end), len(elems)))
		}
		return makeValue("List", elems[int(index):int(end)])
	}
	return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Unsupported sequence operation " + op}
}

// EvalListHigherBuiltinContext preserves compact children and callback result
// identities while traversing a list in the reference's callback order.
func EvalListHigherBuiltinContext(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
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
		return NewRuntimeCompositeContext(ctx, "List", selected)
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

// EvalSetConversionBuiltinContext performs the reference's toSet conversion
// before checking singleton cardinality. Non-enumerable domains expose their
// iteration error; the conversion never guesses from symbolic cardinality.
func EvalSetConversionBuiltinContext(ctx context.Context, op string, args []RuntimeValue) (RuntimeValue, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, err
	}
	if !runtimeHasDomain(args) {
		return EvalSetConversionBuiltin(op, args)
	}
	if setConversionBuiltinArity(op) == 0 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: fmt.Sprintf("Runtime does not support the built -in operator '%s'", op)}
	}
	if len(args) != 1 {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: op + " requires 1 argument"}
	}
	set, err := runtimeToSetContext(ctx, args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	return EvalSetConversionBuiltin(op, []RuntimeValue{set})
}
