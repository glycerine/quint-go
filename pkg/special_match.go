package quint

import (
	"fmt"
	"strings"
)

// MatchConstraints validates literal labels and unary eliminators, then relates
// the scrutinee to the case sum and every eliminator result to the match result.
// Validation returns all case errors in order.
func MatchConstraints(id ID, args []TypedExpression, result Type) ([]Constraint, []ErrorTree) {
	errors := []ErrorTree{}
	labels := args
	if len(args) > 0 {
		labels = args[1:]
	}
	printed := []string{}
	for _, a := range labels {
		printed = append(printed, ExpressionString(a.Expr))
	}
	fail := func(message string) {
		errors = append(errors, ErrorLeaf("Generating match constraints for "+strings.Join(printed, ","), message))
	}
	if len(args) == 0 || len(args)%2 != 1 {
		fail("matchVariant requires a scrutinee followed by label/eliminator pairs")
		return nil, errors
	}
	fields := []RowField{}
	wildcard := false
	for i := 1; i < len(args); i += 2 {
		label, elim := args[i].Expr, args[i+1]
		switch {
		case label.Kind != "str":
			fail("Match variant name must be a string literal but it is a " + label.Kind + ": " + ExpressionString(label))
		case elim.Type.Kind != "oper":
			fail("Match case eliminator must be an operator expression but it is a " + elim.Type.Kind + ": " + ExpressionString(elim.Expr))
		case len(elim.Type.Args) != 1:
			fail(fmt.Sprintf("Match case eliminator must be a unary operator but it is an operator of %d arguments: %s", len(elim.Type.Args), ExpressionString(elim.Expr)))
		case !wildcard && label.Str == "_":
			wildcard = true
		case wildcard:
			fail("Invalid wildcard match ('_') in match expression: " + ExpressionString(elim.Expr) + ". Only one wildcard can appear and it must be the final case of a match.")
		default:
			fields = append(fields, RowField{Name: label.Str, Type: elim.Type.Args[0]})
		}
	}
	if len(errors) > 0 {
		return nil, errors
	}
	tail := Row{Kind: "empty"}
	if wildcard {
		tail = Row{Kind: "var", Name: result.Name + "_wildcard"}
	}
	sum := Type{Kind: "sum", Fields: &Row{Kind: "row", Fields: fields, Other: &tail}}
	constraints := []Constraint{{Kind: "eq", SourceID: id, Types: [2]Type{args[0].Type, sum}}}
	for i := 2; i < len(args); i += 2 {
		constraints = append(constraints, Constraint{Kind: "eq", SourceID: id, Types: [2]Type{result, *args[i].Type.Res}})
	}
	return constraints, errors
}
