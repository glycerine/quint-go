package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
)

func setBuiltinArity(op string) int {
	switch op {
	case "size", "isFinite", "flatten":
		return 1
	case "contains", "in", "subseteq", "exclude", "union", "intersect":
		return 2
	}
	return 0
}

// EvalSetBuiltin evaluates operations on explicit finite sets. It uses exact
// structural keys, independently of locale-dependent display ordering. Symbolic
// and infinite collections are handled separately by the full evaluator.
func EvalSetBuiltin(op string, args []RuntimeValue) (RuntimeValue, error) {
	fail := func(message string) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: message}
	}
	arity := setBuiltinArity(op)
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
	if op == "in" {
		args = []RuntimeValue{args[1], args[0]}
	}
	isSet := func(v RuntimeValue) bool { return v.expression.Kind == "app" && v.expression.Opcode == "Set" }
	if !isSet(args[0]) {
		return fail(op + " requires a Set value")
	}
	boolean := func(value bool) (RuntimeValue, error) { return NewRuntimeValue(Expr{Kind: "bool", Bool: value}) }
	left := args[0].expression.Args
	if op == "size" {
		n, _ := ParseInteger(fmt.Sprint(len(left)))
		return NewRuntimeValue(Expr{Kind: "int", Int: n})
	}
	if op == "isFinite" {
		return boolean(true)
	}
	if op == "contains" || op == "in" {
		for _, element := range left {
			if normalizedValueKey(element) == args[1].identity {
				return boolean(true)
			}
		}
		return boolean(false)
	}
	output := insdict.NewDict[string, Expr]()
	if op == "flatten" {
		for _, element := range left {
			if element.Kind != "app" || element.Opcode != "Set" {
				return fail("flatten requires a Set of Set values")
			}
			for _, child := range element.Args {
				output.Set(normalizedValueKey(child), child)
			}
		}
	} else {
		if !isSet(args[1]) {
			return fail(op + " requires Set values")
		}
		right := map[string]bool{}
		for _, element := range args[1].expression.Args {
			right[normalizedValueKey(element)] = true
		}
		for _, element := range left {
			key := normalizedValueKey(element)
			present := right[key]
			if op == "subseteq" && !present {
				return boolean(false)
			}
			if op == "union" || (op == "exclude" && !present) || (op == "intersect" && present) {
				output.Set(key, element)
			}
		}
		if op == "subseteq" {
			return boolean(true)
		}
		if op == "union" {
			for _, element := range args[1].expression.Args {
				output.Set(normalizedValueKey(element), element)
			}
		}
	}
	values := []Expr{}
	for _, element := range output.All() {
		values = append(values, element)
	}
	return NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: values})
}
