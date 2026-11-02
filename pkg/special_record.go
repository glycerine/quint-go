package quint

import (
	"fmt"
	"strings"
)

type TypedExpression struct {
	Expr Expr
	Type Type
}

func recordConstraintLocation(args []TypedExpression) string {
	printed := make([]string, len(args))
	for index, arg := range args {
		printed[index] = ExpressionString(arg.Expr)
	}
	return "Generating record constraints for " + strings.Join(printed, ",")
}

// RecordConstructorConstraints validates every field name, retaining all
// failures in argument order. No partial constraints are returned on failure.
func RecordConstructorConstraints(id ID, args []TypedExpression, result Type) ([]Constraint, []ErrorTree) {
	errors := []ErrorTree{}
	fail := func(message string) {
		errors = append(errors, ErrorLeaf(recordConstraintLocation(args), message))
	}
	if len(args)%2 != 0 {
		fail("Record constructor requires field/value pairs")
		return nil, errors
	}
	fields := make([]RowField, 0, len(args)/2)
	for index := 0; index < len(args); index += 2 {
		key := args[index].Expr
		if key.Kind != "str" {
			fail("Record field name must be a name expression but is " + key.Kind + ": " + ExpressionString(key))
		} else {
			fields = append(fields, RowField{Name: key.Str, Type: args[index+1].Type})
		}
	}
	if len(errors) > 0 {
		return nil, errors
	}
	record := Type{Kind: "rec", Fields: &Row{Kind: "row", Fields: fields, Other: &Row{Kind: "empty"}}}
	return []Constraint{{Kind: "eq", SourceID: id, Types: [2]Type{record, result}}}, errors
}

// RecordConstraints generates structural constraints for Rec, field,
// fieldNames, and with. Arguments must already have inferred types. Multiple
// invalid constructor fields are returned as children of one error tree.
func RecordConstraints(op string, id ID, args []TypedExpression, result Type) ([]Constraint, *ErrorTree) {
	eq := func(a, b Type) Constraint { return Constraint{Kind: "eq", SourceID: id, Types: [2]Type{a, b}} }
	fail := func(message string) ([]Constraint, *ErrorTree) {
		e := ErrorLeaf(recordConstraintLocation(args), message)
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
		constraints, errors := RecordConstructorConstraints(id, args, result)
		if len(errors) == 1 {
			return nil, &errors[0]
		} else if len(errors) > 1 {
			return nil, &ErrorTree{Location: recordConstraintLocation(args), Children: errors}
		}
		return constraints, nil
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
