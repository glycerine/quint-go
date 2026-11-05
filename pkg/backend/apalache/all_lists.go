package apalache

import (
	"fmt"
	"strings"

	quint "github.com/glycerine/quint-go/pkg"
)

// adaptAllListsUpTo rejects every allListsUpTo application in the module, as
// upstream Quint does for CHECK, TLA and verify input.
func adaptAllListsUpTo(compiled quint.CompilationResult) (quint.CompilationResult, error) {
	return rejectDefinedOperator(compiled, "allListsUpTo")
}

// adaptFoldRight rejects every foldr application, as upstream does for CHECK and
// TLA input: the Apalache QuintIR parser has no conversion for the operator.
func adaptFoldRight(compiled quint.CompilationResult) (quint.CompilationResult, error) {
	return rejectDefinedOperator(compiled, "foldr")
}

// rejectDefinedOperator fails on any application of op, including one in an
// unused definition. The diagnostic keeps upstream's wording. The lookup ID and
// the argument rendering match upstream on the probed specs.
func rejectDefinedOperator(compiled quint.CompilationResult, op string) (quint.CompilationResult, error) {
	transformer := &quint.IRTransformer{ExitExpression: func(expression quint.Expr) (quint.Expr, error) {
		if expression.Kind != "app" || expression.Opcode != op {
			return expression, nil
		}
		return quint.Expr{}, fmt.Errorf("internal error: while parsing in Apalache:\n'Input was not a valid representation of the QuintIR: While converting operator application of defined operator '%s' to arguments %s: Input was not a valid representation of the QuintIR: No entry found for id %s in lookup table'\nPlease report an issue: https://github.com/informalsystems/quint/issues/new", op, upstreamIRList(expression.Args), expression.ID.String())
	}}
	if _, err := transformer.TransformModule(compiled.MainModule); err != nil {
		return compiled, err
	}
	return compiled, nil
}

// upstreamIRList renders arguments in the QuintIR form upstream prints in its
// diagnostic, for example List(QuintApp(7,Set,List(QuintInt(5,1), QuintInt(6,2))), QuintInt(8,2)).
func upstreamIRList(args []quint.Expr) string {
	parts := make([]string, len(args))
	for i, arg := range args {
		parts[i] = upstreamIR(arg)
	}
	return "List(" + strings.Join(parts, ", ") + ")"
}

func upstreamIR(e quint.Expr) string {
	id := e.ID.String()
	switch e.Kind {
	case "name":
		return fmt.Sprintf("QuintName(%s,%s)", id, e.Name)
	case "int":
		return fmt.Sprintf("QuintInt(%s,%s)", id, e.Int.String())
	case "str":
		return fmt.Sprintf("QuintStr(%s,%s)", id, e.Str)
	case "bool":
		return fmt.Sprintf("QuintBool(%s,%t)", id, e.Bool)
	case "app":
		return fmt.Sprintf("QuintApp(%s,%s,%s)", id, e.Opcode, upstreamIRList(e.Args))
	case "lambda":
		params := make([]string, len(e.Params))
		for i, param := range e.Params {
			params[i] = fmt.Sprintf("QuintLambdaParameter(%s,%s)", param.ID.String(), param.Name)
		}
		body := ""
		if e.Body != nil {
			body = upstreamIR(*e.Body)
		}
		return fmt.Sprintf("QuintLambda(%s,List(%s),%s,%s)", id, strings.Join(params, ", "), string(e.Qualifier), body)
	}
	return quint.ExpressionString(e)
}
