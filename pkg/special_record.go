package quint

import (
	"fmt"
	"strings"
)

type TypedExpression struct {
	Expr Expr
	Type Type
}

// RecordConstraints generates structural constraints for Rec, field,
// fieldNames, and with. Arguments must already have inferred types.
func RecordConstraints(op string, id ID, args []TypedExpression, result Type) ([]Constraint, *ErrorTree) {
	eq := func(a, b Type) Constraint { return Constraint{Kind: "eq", SourceID: id, Types: [2]Type{a, b}} }
	fail := func(message string) ([]Constraint, *ErrorTree) {
		printed := []string{}
		for _, a := range args {
			printed = append(printed, ExpressionString(a.Expr))
		}
		e := ErrorLeaf("Generating record constraints for "+strings.Join(printed, ","), message)
		return nil, &e
	}
	rowType := func(fields []RowField, tail Row) Type {
		return Type{Kind: "rec", Fields: &Row{Kind: "row", Fields: fields, Other: &tail}}
	}
	required := map[string]int{"field": 2, "fieldNames": 1, "with": 3}
	if n, ok := required[op]; ok && len(args) != n {
		return fail(fmt.Sprintf("%s expects %d arguments", op, n))
	}
	if op == "Rec" {
		if len(args)%2 != 0 {
			return fail("Record constructor requires field/value pairs")
		}
		fields := []RowField{}
		for i := 0; i < len(args); i += 2 {
			key := args[i].Expr
			if key.Kind != "str" {
				return fail("Record field name must be a name expression but is " + key.Kind + ": " + ExpressionString(key))
			}
			fields = append(fields, RowField{Name: key.Str, Type: args[i+1].Type})
		}
		return []Constraint{eq(rowType(fields, Row{Kind: "empty"}), result)}, nil
	}
	if op == "fieldNames" {
		str := Type{Kind: "str"}
		return []Constraint{eq(result, Type{Kind: "set", Elem: &str}), eq(args[0].Type, Type{Kind: "rec", Fields: &Row{Kind: "var", Name: "rec_" + result.Name}})}, nil
	}
	if op == "field" || op == "with" {
		key := args[1].Expr
		if key.Kind != "str" {
			return fail("Record field name must be a string expression but is " + key.Kind + ": " + ExpressionString(key))
		}
		fieldType := result
		if op == "with" {
			fieldType = args[2].Type
		}
		rec := rowType([]RowField{{Name: key.Str, Type: fieldType}}, Row{Kind: "var", Name: "tail_" + result.Name})
		constraints := []Constraint{eq(args[0].Type, rec)}
		if op == "with" {
			constraints = append(constraints, eq(result, rec))
		}
		return constraints, nil
	}
	return fail("Unknown record operator " + op)
}
