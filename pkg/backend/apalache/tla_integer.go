package apalache

import (
	"math/big"

	"github.com/glycerine/insdict"
	quint "github.com/glycerine/quint-go/pkg"
)

// adaptTLAIntegerDivision rewrites signed Quint quotient/remainder before the
// Go Apalache TLA printer emits TLA integer operators. TLA's \div rounds
// toward negative infinity; Quint division truncates toward zero. The rewrite
// is local to TLA output and does not change the assignment IR used by CHECK.
func adaptTLAIntegerDivision(compiled quint.CompilationResult) (quint.CompilationResult, error) {
	if compiled.Analysis.Types == nil {
		return compiled, nil
	}
	maximum := new(big.Int)
	for key := range compiled.Analysis.Types.All() {
		value, ok := new(big.Int).SetString(key, 10)
		if ok && value.Cmp(maximum) > 0 {
			maximum.Set(value)
		}
	}
	last, err := quint.ParseID(maximum.String())
	if err != nil {
		return compiled, err
	}
	ids := quint.NewIDGeneratorAfter(last)
	types := insdict.NewDict[string, quint.TypeScheme]()
	for key, scheme := range compiled.Analysis.Types.All() {
		types.Set(key, scheme)
	}
	newExpr := func(kind string, opcode string, args ...quint.Expr) quint.Expr {
		id := ids.Next()
		typ := "int"
		switch opcode {
		case "ilt", "igt", "and", "or":
			typ = "bool"
		}
		types.Set(id.String(), quint.TypeScheme{Type: quint.Type{Kind: typ}, TypeVariables: []string{}, RowVariables: []string{}})
		if kind == "int" {
			zero, _ := quint.ParseInteger("0")
			return quint.Expr{ID: id, Kind: "int", Int: zero}
		}
		return quint.Expr{ID: id, Kind: "app", Opcode: opcode, Args: args}
	}
	intZero := func() quint.Expr { return newExpr("int", "") }
	app := func(op string, args ...quint.Expr) quint.Expr { return newExpr("app", op, args...) }
	root := func(id quint.ID, op string, args ...quint.Expr) quint.Expr {
		return quint.Expr{ID: id, Kind: "app", Opcode: op, Args: args}
	}
	truncatingQuotient := func(x, y quint.Expr, target quint.ID, reuseTarget bool) quint.Expr {
		zeroForSign := intZero()
		xNegative := app("ilt", x, zeroForSign)
		zeroForY := intZero()
		yNegative := app("ilt", y, zeroForY)
		xAbs := app("ite", xNegative, app("iuminus", x), x)
		yAbs := app("ite", yNegative, app("iuminus", y), y)
		magnitude := app("idiv", xAbs, yAbs)
		xPositive := app("igt", x, intZero())
		yPositive := app("igt", y, intZero())
		oppositeSigns := app("or",
			app("and", xNegative, yPositive),
			app("and", xPositive, yNegative),
		)
		result := app("ite", oppositeSigns, app("iuminus", magnitude), magnitude)
		if reuseTarget {
			types.Del(result.ID.String())
			result.ID = target
		}
		return result
	}
	transformer := &quint.IRTransformer{ExitExpression: func(expression quint.Expr) (quint.Expr, error) {
		if expression.Kind != "app" || len(expression.Args) != 2 {
			return expression, nil
		}
		x, y := expression.Args[0], expression.Args[1]
		switch expression.Opcode {
		case "idiv":
			return truncatingQuotient(x, y, expression.ID, true), nil
		case "imod":
			quotient := truncatingQuotient(x, y, quint.ID{}, false)
			product := app("imul", quotient, y)
			return root(expression.ID, "isub", x, product), nil
		default:
			return expression, nil
		}
	}}
	module, err := transformer.TransformModule(compiled.MainModule)
	if err != nil {
		return compiled, err
	}
	compiled.MainModule = module
	compiled.Analysis.Types = types
	return compiled, nil
}
