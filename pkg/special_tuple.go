package quint

import (
	"math/big"
	"strconv"
	"strings"
)

// TupleConstructorConstraints assigns zero-based field labels in argument order.
func TupleConstructorConstraints(id ID, args []TypedExpression, result Type) []Constraint {
	fields := make([]RowField, len(args))
	for i, arg := range args {
		fields[i] = RowField{Name: strconv.Itoa(i), Type: arg.Type}
	}
	tuple := Type{Kind: "tup", Fields: &Row{Kind: "row", Fields: fields, Other: &Row{Kind: "empty"}}}
	return []Constraint{{Kind: "eq", SourceID: id, Types: [2]Type{tuple, result}}}
}

// TupleItemConstraints describes a tuple prefix ending at the one-based literal
// index. Its open tail permits additional fields. Prefixes with more than one
// million preceding fields return a resource-limit diagnostic instead of
// allocating unbounded memory. Upstream eagerly allocates these fields without
// a resource bound; this diagnostic is an explicit Go resource policy.
func TupleItemConstraints(id ID, args []TypedExpression, result Type) ([]Constraint, *ErrorTree) {
	fail := func(message string) ([]Constraint, *ErrorTree) {
		printed := []string{}
		for _, a := range args {
			printed = append(printed, ExpressionString(a.Expr))
		}
		e := ErrorLeaf("Generating record constraints for "+strings.Join(printed, ","), message)
		return nil, &e
	}
	if len(args) != 2 {
		return fail("item expects 2 arguments")
	}
	index := args[1].Expr
	if index.Kind != "int" {
		return fail("Tup field index must be an int expression but is " + index.Kind + ": " + ExpressionString(index))
	}
	previous := new(big.Int).Sub(index.Int.Big(), big.NewInt(1))
	fields := []RowField{}
	// Match lodash's empty result for values beyond MAX_SAFE_INTEGER.
	if previous.Cmp(big.NewInt(1000000)) > 0 && previous.Cmp(big.NewInt(9007199254740991)) <= 0 {
		return fail("Tuple projection exceeds the supported inference prefix of 1000000 fields")
	}
	if previous.Sign() > 0 && previous.Cmp(big.NewInt(9007199254740991)) <= 0 {
		// Materialization is proportional to the requested prefix; callers should
		// reject impractically large projections before invoking inference.
		for i := int64(0); i < previous.Int64(); i++ {
			n := strconv.FormatInt(i, 10)
			fields = append(fields, RowField{Name: n, Type: Type{Kind: "var", Name: "tup_" + result.Name + "_" + n}})
		}
	}
	fields = append(fields, RowField{Name: previous.String(), Type: result})
	tuple := Type{Kind: "tup", Fields: &Row{Kind: "row", Fields: fields, Other: &Row{Kind: "var", Name: "tail_" + result.Name}}}
	return []Constraint{{Kind: "eq", SourceID: id, Types: [2]Type{args[0].Type, tuple}}}, nil
}
