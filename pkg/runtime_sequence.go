package quint

import (
	"encoding/json"
	"fmt"
	"math"
)

func sequenceBuiltinArity(op string) int {
	switch op {
	case "head", "tail", "length", "fieldNames", "indices":
		return 1
	case "item", "nth", "append", "concat", "field":
		return 2
	case "replaceAt", "slice", "with":
		return 3
	}
	return 0
}

// EvalSequenceBuiltin evaluates strict sequence/tuple/record operations on owned
// runtime values. Indexing follows JavaScript Number conversion; tuple positions start at one,
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
	elems, err := runtimeChildren(args[0])
	if err != nil {
		return RuntimeValue{}, err
	}
	makeValue := newRuntimeComposite
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
		indices := make([]RuntimeValue, len(elems))
		for i := range elems {
			n, _ := ParseInteger(fmt.Sprint(i))
			indices[i], err = NewRuntimeValue(Expr{Kind: "int", Int: n})
			if err != nil {
				return RuntimeValue{}, err
			}
		}
		return makeValue("Set", indices)
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
	panic("unreachable sequence builtin")
}

// JSON's float formatting uses the same decimal/exponent cutoffs as JavaScript.
func runtimeNumberString(n float64) string {
	if math.IsInf(n, 1) {
		return "Infinity"
	}
	if math.IsInf(n, -1) {
		return "-Infinity"
	}
	encoded, _ := json.Marshal(n)
	return string(encoded)
}
