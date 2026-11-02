package quint

import (
	"fmt"
	"strconv"
)

//go:generate node grammar/builtin_types_dynamic_ids.cjs

// dynamicBuiltinTypeScheme builds variable-arity builtin signatures directly.
// Every argument/result subtree is fresh; no caller-selected arity is cached.
// IDs follow the reference parser's postorder allocation, starting anew per call.
func dynamicBuiltinTypeScheme(name string, arity int) (TypeScheme, error) {
	if arity < 0 {
		return TypeScheme{}, fmt.Errorf("negative builtin arity")
	}
	if name == "matchVariant" && (arity < 1 || arity%2 == 0) {
		return TypeScheme{}, fmt.Errorf("matchVariant requires a value and pattern/function pairs")
	}
	switch name {
	case "List", "Set", "Map", "and", "or", "actionAll", "actionAny", "matchVariant", "tuples":
	default:
		return TypeScheme{}, fmt.Errorf("no ordinary builtin signature for %q", name)
	}
	nextID := 1
	withID := func(typ Type) Type {
		id := ID{value: Integer{decimal: strconv.Itoa(nextID)}}
		nextID++
		typ.ID = &id
		return typ
	}
	variable := func(name string) Type { return withID(Type{Kind: "var", Name: name}) }
	arguments := make([]Type, arity)
	variables := []string{}
	var result Type
	switch name {
	case "List", "Set":
		for i := range arguments {
			arguments[i] = variable("a")
		}
		element := variable("a")
		kind := "list"
		if name == "Set" {
			kind = "set"
		}
		result = withID(Type{Kind: kind, Elem: &element})
		variables = []string{"a"}
	case "Map":
		for i := range arguments {
			arguments[i] = withID(Type{Kind: "tup", Fields: &Row{Kind: "row", Fields: []RowField{
				{Name: "0", Type: variable("a")}, {Name: "1", Type: variable("b")},
			}, Other: &Row{Kind: "empty"}}})
		}
		key, value := variable("a"), variable("b")
		result = withID(Type{Kind: "fun", Arg: &key, Res: &value})
		variables = []string{"a", "b"}
	case "and", "or", "actionAll", "actionAny":
		for i := range arguments {
			arguments[i] = withID(Type{Kind: "bool"})
		}
		result = withID(Type{Kind: "bool"})
	case "matchVariant":
		arguments[0] = variable("a")
		for i := 1; i < arity; i += 2 {
			arguments[i] = withID(Type{Kind: "str"})
			argument := variable("a")
			value := variable("b")
			arguments[i+1] = withID(Type{Kind: "oper", Args: []Type{argument}, Res: &value})
		}
		result = variable("b")
		variables = []string{"a", "b"}
	case "tuples":
		variables = make([]string, arity)
		for i := range arguments {
			variables[i] = "t" + strconv.Itoa(i)
			element := variable(variables[i])
			arguments[i] = withID(Type{Kind: "set", Elem: &element})
		}
		var product Type
		switch arity {
		case 0:
			product = withID(Type{Kind: "tup", Fields: &Row{Kind: "empty"}})
		case 1:
			// The original generated signature Set[(t0)] parses as Set[t0].
			product = variable("t0")
		default:
			fields := make([]RowField, arity)
			for i := range fields {
				fields[i] = RowField{Name: strconv.Itoa(i), Type: variable(variables[i])}
			}
			product = withID(Type{Kind: "tup", Fields: &Row{Kind: "row", Fields: fields, Other: &Row{Kind: "empty"}}})
		}
		result = withID(Type{Kind: "set", Elem: &product})
	default:
		return TypeScheme{}, fmt.Errorf("no ordinary builtin signature for %q", name)
	}
	return TypeScheme{Type: withID(Type{Kind: "oper", Args: arguments, Res: &result}), TypeVariables: variables, RowVariables: []string{}}, nil
}
