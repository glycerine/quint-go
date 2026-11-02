package quint

import "strings"

// VariantConstraints requires a literal label, propagates the payload type, and
// requires an enclosing declared sum type. The solver processes the equality
// before the defined-type check, despite their generated order.
func VariantConstraints(id ID, args []TypedExpression, result Type) ([]Constraint, *ErrorTree) {
	fail := func(message string) ([]Constraint, *ErrorTree) {
		expressions := []string{}
		for _, a := range args {
			expressions = append(expressions, ExpressionString(a.Expr))
		}
		e := ErrorLeaf("Generating variant constraints for "+strings.Join(expressions, ","), message)
		return nil, &e
	}
	if len(args) != 2 {
		return fail("variant expects 2 arguments")
	}
	label := args[0].Expr
	if label.Kind != "str" {
		return fail("Variant label must be a string expression but is " + args[0].Type.Kind + ": " + ExpressionString(label))
	}
	variant := Type{Kind: "sum", Fields: &Row{Kind: "row", Fields: []RowField{{Name: label.Str, Type: args[1].Type}}, Other: &Row{Kind: "var", Name: "tail_" + result.Name}}}
	return []Constraint{
		{Kind: "isDefined", SourceID: id, Type: variant},
		{Kind: "eq", SourceID: id, Types: [2]Type{result, variant}},
	}, nil
}
