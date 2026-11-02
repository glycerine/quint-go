package quint

import (
	"fmt"
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
	"math/big"
	"regexp"
	"strings"
)

// ParseExpression parses a complete expression. Conversion support is added
// incrementally; unsupported syntax returns an explicit error, never partial IR.
func ParseExpression(text string) (Expr, error) {
	return parseExpressionWithIDs(text, NewIDGenerator())
}
func parseExpressionWithIDs(text string, ids parserIDAllocator) (Expr, error) {
	listener := newSyntaxErrors(text)
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser := p.NewQuintParser(tokens)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	tree := parser.Expr()
	if tokens.LA(1) != antlr.TokenEOF {
		listener.trailing(tokens.LT(1))
	}
	if len(listener.issues) > 0 {
		return Expr{}, listener.err("expression")
	}
	return expressionFromContext(tree, ids)
}
func expressionFromContext(ctx p.IExprContext, ids parserIDAllocator) (Expr, error) {
	if recoveringSyntax(ids) {
		if _, missing := ctx.(*p.ExprContext); missing {
			return Expr{}, nil
		}
		if unsupportedRecoveryOperands(ctx, ids) && !recoveryOperandReducerSupported(ctx) {
			return Expr{}, unexpectedRecoveryNode(ctx)
		}
	}
	defer parserScope(ids, ctx)()
	if c, ok := ctx.(*p.LiteralOrIdContext); ok {
		e := Expr{ID: ids.Next()}
		switch {
		case c.QualId() != nil:
			e.Kind = "name"
			e.Name = c.GetText()
		case c.INT() != nil:
			e.Kind = "int"
			literal := strings.ReplaceAll(c.GetText(), "_", "")
			base := 10
			if strings.HasPrefix(literal, "0x") {
				base, literal = 16, literal[2:]
			}
			n, ok := new(big.Int).SetString(literal, base)
			if !ok {
				return Expr{}, fmt.Errorf("invalid integer literal %q", c.GetText())
			}
			e.Int = IntegerFromBig(n)
		case c.BOOL() != nil:
			e.Kind = "bool"
			e.Bool = c.GetText() == "true"
		case c.STRING() != nil:
			e.Kind = "str"
			e.Str = c.GetText()[1 : len(c.GetText())-1]
		}
		return e, nil
	}
	if c, ok := ctx.(*p.MatchContext); ok {
		match := c.MatchSumExpr()
		scrutinee, err := expressionFromContext(match.Expr(), ids)
		if err != nil {
			return Expr{}, err
		}
		arms := match.AllMatchSumCase()
		bodies := make([]Expr, len(arms))
		for i, arm := range arms {
			body, err := expressionFromContext(arm.Expr(), ids)
			if err != nil {
				return Expr{}, err
			}
			bodies[i] = body
		}
		// The listener visits every arm body before lowering the match into
		// its application and synthetic label/lambda/parameter nodes.
		matchID := parserIDAt(ids, match)
		args := make([]Expr, 1, 1+2*len(arms))
		args[0] = scrutinee
		for i, arm := range arms {
			label, name := "_", "_"
			if variant := arm.GetVariantMatch(); variant != nil {
				label = variant.GetVariantLabel().GetText()
				if variant.GetVariantParam() != nil {
					name = variant.GetVariantParam().GetText()
				}
			}
			labelExpr := Expr{ID: parserIDAt(ids, arm), Kind: "str", Str: label}
			elimID := parserIDAt(ids, arm)
			parameter := Parameter{ID: parserIDAt(ids, arm), Name: name}
			elim := Expr{ID: elimID, Kind: "lambda", Qualifier: Def, Params: []Parameter{parameter}, Body: &bodies[i]}
			args = append(args, labelExpr, elim)
		}
		return Expr{ID: matchID, Kind: "app", Opcode: "matchVariant", Args: args}, nil
	}
	if c, ok := ctx.(*p.LambdaConsContext); ok {
		lambda := c.Lambda().LambdaUnsugared()
		if lambda == nil {
			tuple := c.Lambda().LambdaTupleSugar()
			params := []Parameter{}
			for _, param := range tuple.AllParameter() {
				params = append(params, Parameter{ID: parserIDAt(ids, param), Name: param.GetText()})
			}
			body, err := expressionFromContext(tuple.Expr(), ids)
			if err != nil {
				return Expr{}, err
			}
			body = recoveryExpressionBody(ids, tuple, body)
			freshID := ids.Next()
			fresh := Parameter{ID: freshID, Name: "quintTupledLambdaParam" + freshID.String()}
			// Upstream reduces left to right: the last binding encloses the others.
			for i, param := range params {
				index, _ := ParseInteger(fmt.Sprint(i + 1))
				access := Expr{ID: ids.Next(), Kind: "app", Opcode: "item", Args: []Expr{
					{ID: ids.Next(), Kind: "name", Name: fresh.Name},
					{ID: ids.Next(), Kind: "int", Int: index},
				}}
				inner := body
				body = Expr{ID: ids.Next(), Kind: "let", Opdef: &OperatorDef{ID: param.ID, Kind: "def", Name: param.Name, Qualifier: PureVal, Expr: &access}, Body: &inner}
			}
			return Expr{ID: ids.Next(), Kind: "lambda", Params: []Parameter{fresh}, Qualifier: Def, Body: &body}, nil
		}
		params := []Parameter{}
		for _, param := range lambda.AllParameter() {
			params = append(params, Parameter{ID: parserIDAt(ids, param), Name: param.GetText()})
		}
		body, err := expressionFromContext(lambda.Expr(), ids)
		if err != nil {
			return Expr{}, err
		}
		body = recoveryExpressionBody(ids, lambda, body)
		return Expr{ID: ids.Next(), Kind: "lambda", Params: params, Qualifier: Def, Body: &body}, nil
	}
	if c, ok := ctx.(*p.RecordContext); ok {
		pairs := []Expr{}
		var spread *Expr
		seen := map[string]bool{}
		duplicateSet := map[string]bool{}
		duplicates := []string{}
		spreadCount := 0
		for _, elem := range c.AllRecElem() {
			value, err := expressionFromContext(elem.Expr(), ids)
			if err != nil {
				return Expr{}, err
			}
			value = recoveryExpressionBody(ids, elem, value)
			if elem.SimpleId() == nil {
				spreadCount++
				spread = &value
				continue
			}
			name := elem.SimpleId().GetText()
			if seen[name] && !duplicateSet[name] {
				duplicates = append(duplicates, name)
				duplicateSet[name] = true
			}
			seen[name] = true
			pairs = append(pairs, Expr{ID: parserIDAt(ids, elem), Kind: "str", Str: name}, value)
		}
		if len(duplicates) > 0 {
			plural := "Field"
			if len(duplicates) > 1 {
				plural = "Fields"
			}
			id := ids.Next()
			diagnostic := Diagnostic{Code: QNT016, Message: plural + " " + strings.Join(duplicates, ", ") + " cannot be defined more than once", Reference: &id, Data: &DiagnosticData{}}
			if !reportParserDiagnostic(ids, diagnostic) {
				return Expr{}, diagnostic
			}
		}
		if spreadCount > 1 {
			id := ids.Next()
			diagnostic := Diagnostic{Code: QNT012, Message: "... may be used once in { ...record, <fields> }", Reference: &id, Data: &DiagnosticData{}}
			if reportParserDiagnostic(ids, diagnostic) {
				return parserUndefinedExpression(ids), nil
			}
			return Expr{}, diagnostic
		}
		if spread == nil {
			return Expr{ID: ids.Next(), Kind: "app", Opcode: "Rec", Args: pairs}, nil
		}
		record := *spread
		for i := 0; i < len(pairs); i += 2 {
			record = Expr{ID: ids.Next(), Kind: "app", Opcode: "with", Args: []Expr{record, pairs[i], pairs[i+1]}}
		}
		return record, nil
	}
	if c, ok := ctx.(*p.LetInContext); ok {
		defs, err := destructuredOperators(c.OperDef(), ids)
		if err != nil {
			return Expr{}, err
		}
		if defs != nil {
			body, err := expressionFromContext(c.Expr(), ids)
			if err != nil {
				return Expr{}, err
			}
			body = recoveryExpressionBody(ids, c, body)
			for _, op := range defs[1:] {
				inner := body
				body = Expr{ID: ids.Next(), Kind: "let", Opdef: &op, Body: &inner}
			}
			return Expr{ID: ids.Next(), Kind: "let", Opdef: &defs[0], Body: &body}, nil
		}

		definition, err := operatorFromContext(c.OperDef(), ids)
		if err != nil {
			return Expr{}, err
		}
		body, err := expressionFromContext(c.Expr(), ids)
		if err != nil {
			return Expr{}, err
		}
		body = recoveryExpressionBody(ids, c, body)
		return Expr{ID: ids.Next(), Kind: "let", Opdef: &definition, Body: &body}, nil
	}
	args := []Expr{}
	defer recoveryOperandFrame(ids, &args)()
	required := 0
	for _, child := range ctx.GetChildren() {
		if ex, ok := child.(p.IExprContext); ok {
			required++
			v, err := expressionFromContext(ex, ids)
			if err != nil {
				return Expr{}, err
			}
			if v.Kind != "" {
				args = append(args, v)
			}
		}
	}
	appendArgs := func(list p.IArgListContext) error {
		if list == nil {
			return nil
		}
		values := []Expr{}
		restore := recoveryOperandFrame(ids, &values)
		defer restore()
		for _, ctx := range list.AllExpr() {
			v, err := expressionFromContext(ctx, ids)
			if err != nil {
				return err
			}
			if v.Kind != "" {
				values = append(values, v)
			}
		}
		if recoveringSyntax(ids) {
			values = recoveryTakeOperands(ids.(*recoveringParserIDs), len(list.AllExpr()), list)
		}
		required += len(list.AllExpr())
		args = append(args, values...)
		return nil
	}
	opcode := ""
	switch c := ctx.(type) {
	case *p.ParenContext, *p.BracesContext:
		if len(args) == 0 && recoveringSyntax(ids) {
			return Expr{}, nil
		}
		return args[0], nil
	case *p.OperAppContext:
		opcode = c.NormalCallName().GetText()
		if err := appendArgs(c.ArgList()); err != nil {
			return Expr{}, err
		}
	case *p.DotCallContext:
		name := c.NameAfterDot().GetText()
		if c.LPAREN() != nil {
			opcode = name
			if err := appendArgs(c.ArgList()); err != nil {
				return Expr{}, err
			}
		} else {
			index := tupleFieldPattern.FindStringSubmatch(name)
			field := Expr{ID: ids.Next(), Kind: "str", Str: name}
			opcode = "field"
			if index != nil {
				n, _ := ParseInteger(index[1])
				field.Kind = "int"
				field.Int = n
				field.Str = ""
				opcode = "item"
			}
			args = append(args, field)
		}
	case *p.ListAppContext:
		opcode = "nth"
	case *p.AsgnContext:
		opcode = "assign"
		if recoveringSyntax(ids) {
			lhs := Expr{ID: ids.Next(), Kind: "name", Name: c.QualId().GetText()}
			args = recoveryTakeOperands(ids.(*recoveringParserIDs), 1, ctx)
			args = append([]Expr{lhs}, args...)
			break
		}
		args = append([]Expr{{ID: ids.Next(), Kind: "name", Name: c.QualId().GetText()}}, args...)
	case *p.UminusContext:
		opcode = "iuminus"
	case *p.PowContext:
		opcode = "ipow"
	case *p.PlusMinusContext:
		opcode = map[string]string{"+": "iadd", "-": "isub"}[c.GetOp().GetText()]
	case *p.MultDivContext:
		opcode = map[string]string{"*": "imul", "/": "idiv", "%": "imod"}[c.GetOp().GetText()]
	case *p.RelationsContext:
		opcode = map[string]string{"==": "eq", "!=": "neq", ">": "igt", ">=": "igte", "<": "ilt", "<=": "ilte"}[c.GetOp().GetText()]
	case *p.ErrorEqContext:
		opcode = "eq"
	case *p.AndContext, *p.AndExprContext:
		opcode = "and"
	case *p.OrContext, *p.OrExprContext:
		opcode = "or"
	case *p.IffContext:
		opcode = "iff"
	case *p.ImpliesContext:
		opcode = "implies"
	case *p.LeadsToContext:
		opcode = "leadsTo"
	case *p.ActionAllContext:
		opcode = "actionAll"
	case *p.ActionAnyContext:
		opcode = "actionAny"
	case *p.TupleContext, *p.PairContext, *p.UnitContext:
		opcode = "Tup"
	case *p.ListContext:
		opcode = "List"
	case *p.IfElseContext:
		opcode = "ite"
	default:
		return Expr{}, fmt.Errorf("expression conversion not implemented for %T", ctx)
	}
	if recoveringSyntax(ids) && ids.(*recoveringParserIDs).expressionHole(ctx) && opcode != "assign" {
		recovery := ids.(*recoveringParserIDs)
		if opcode == "iuminus" && recoveryOperandCount(recovery) == 0 {
			return Expr{}, nil
		}
		args = recoveryTakeOperands(recovery, required, ctx)
	}
	id := ids.Next()
	if opcode == "q::debug" && len(args) == 1 {
		// Upstream uses parse-tree text, which excludes skipped whitespace and
		// comments while retaining the original token spelling.
		label := Expr{ID: ids.Next(), Kind: "str", Str: ctx.GetText()}
		args = append([]Expr{label}, args...)
	}
	return Expr{ID: id, Kind: "app", Opcode: opcode, Args: args}, nil
}

var tupleFieldPattern = regexp.MustCompile(`^_([1-9][0-9]?)$`)
