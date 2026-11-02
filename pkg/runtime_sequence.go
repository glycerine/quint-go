package quint

import (
	"fmt"
	"math/big"
)

func sequenceBuiltinArity(op string) int {
	switch op {
	case "head", "tail", "length", "fieldNames":
		return 1
	case "item", "nth", "append", "concat", "field":
		return 2
	case "replaceAt", "slice", "with":
		return 3
	}
	return 0
}

// EvalSequenceBuiltin evaluates strict sequence/tuple/record operations on owned
// runtime values. Indexing uses exact integers; tuple positions start at one,
// while list positions and slice endpoints start at zero.
func EvalSequenceBuiltin(op string, args []RuntimeValue) (RuntimeValue, error) {
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
	e := args[0].Expression()
	elems := e.Args
	makeValue := func(op string, elems []Expr) (RuntimeValue, error) {
		return NewRuntimeValue(Expr{Kind: "app", Opcode: op, Args: elems})
	}
	if record {
		if op == "fieldNames" {
			names := []Expr{}
			for i := 0; i < len(elems); i += 2 {
				names = append(names, elems[i])
			}
			return makeValue("Set", names)
		}
		if args[1].expression.Kind != "str" {
			return fail("QNT501", op+" requires a string field")
		}
		name := args[1].expression.Str
		for i := 0; i < len(elems); i += 2 {
			if elems[i].Str == name {
				if op == "field" {
					return NewRuntimeValue(elems[i+1])
				}
				elems[i+1] = args[2].expression
				return makeValue("Rec", elems)
			}
		}
		if op == "field" {
			return fail("QNT501", "Accessing a missing record field "+name)
		}
		return fail("QNT501", "Called 'with' with a non-existent field "+name)
	}
	switch op {
	case "length":
		n, _ := ParseInteger(fmt.Sprint(len(elems)))
		return NewRuntimeValue(Expr{Kind: "int", Int: n})
	case "head", "tail":
		if len(elems) == 0 {
			return fail("QNT505", fmt.Sprintf("Called '%s' on an empty list", op))
		}
		if op == "head" {
			return NewRuntimeValue(elems[0])
		}
		return makeValue("List", elems[1:])
	case "append":
		return makeValue("List", append(elems, args[1].expression))
	case "concat":
		if args[1].expression.Kind != "app" || args[1].expression.Opcode != "List" {
			return fail("QNT501", "concat requires List values")
		}
		return makeValue("List", append(elems, args[1].expression.Args...))
	}
	if args[1].expression.Kind != "int" {
		return fail("QNT501", op+" requires integer indices")
	}
	index := args[1].expression.Int.Big()
	if op == "item" {
		index.Sub(index, big.NewInt(1))
	}
	valid := index.IsInt64() && index.Sign() >= 0 && index.Int64() < int64(len(elems))
	switch op {
	case "item", "nth":
		if !valid {
			return fail("QNT510", "Out of bounds, nth("+index.String()+")")
		}
		return NewRuntimeValue(elems[int(index.Int64())])
	case "replaceAt":
		if !valid {
			return fail("QNT510", "Out of bounds, replaceAt("+index.String()+")")
		}
		elems[int(index.Int64())] = args[2].expression
		return makeValue("List", elems)
	case "slice":
		if args[2].expression.Kind != "int" {
			return fail("QNT501", "slice requires integer indices")
		}
		end := args[2].expression.Int.Big()
		if index.Sign() < 0 || index.Cmp(end) > 0 || !end.IsInt64() || end.Int64() > int64(len(elems)) {
			return fail("QNT506", fmt.Sprintf("slice(..., %s, %s) applied to a list of size %d", index, end, len(elems)))
		}
		return makeValue("List", elems[int(index.Int64()):int(end.Int64())])
	}
	panic("unreachable sequence builtin")
}
