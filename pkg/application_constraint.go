package quint

import "fmt"

// ApplicationConstraints dispatches special builtin rules or relates a standard
// operator signature to argument/result types. signature must already have its
// quantified variables instantiated. Special builtins do not use signature.
func ApplicationConstraints(op string, id ID, args []TypedExpression, result Type, signature Type) ([]Constraint, []ErrorTree) {
	expected := ""
	valid := true
	switch op {
	case "Rec":
		expected = "even number of"
		valid = len(args)%2 == 0
	case "field", "item", "variant":
		expected = "2"
		valid = len(args) == 2
	case "fieldNames":
		expected = "1"
		valid = len(args) == 1
	case "with":
		expected = "3"
		valid = len(args) == 3
	case "matchVariant":
		expected = "odd number of"
		valid = len(args)%2 != 0
	}
	if !valid {
		return nil, []ErrorTree{ErrorLeaf("Checking arity for application of "+op, fmt.Sprintf("Operator expects %s arguments but was given %d", expected, len(args)))}
	}
	var constraints []Constraint
	var failure *ErrorTree
	switch op {
	case "Rec", "field", "fieldNames", "with":
		constraints, failure = RecordConstraints(op, id, args, result)
	case "Tup":
		constraints = TupleConstructorConstraints(id, args, result)
	case "item":
		constraints, failure = TupleItemConstraints(id, args, result)
	case "variant":
		constraints, failure = VariantConstraints(id, args, result)
	case "matchVariant":
		return MatchConstraints(id, args, result)
	default:
		types := make([]Type, len(args))
		for i, a := range args {
			types[i] = a.Type
		}
		constraints = []Constraint{{Kind: "eq", SourceID: id, Types: [2]Type{signature, {Kind: "oper", Args: types, Res: &result}}}}
	}
	if failure != nil {
		return nil, []ErrorTree{*failure}
	}
	return constraints, []ErrorTree{}
}
