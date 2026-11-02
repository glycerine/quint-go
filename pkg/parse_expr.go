package quint

import (
	"fmt"
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
	"regexp"
	"strings"
)

// ParseExpression parses a complete expression. Conversion support is added
// incrementally; unsupported syntax returns an explicit error, never partial IR.
func ParseExpression(text string) (Expr, error) {
	listener := &effectSyntaxErrors{DefaultErrorListener: antlr.NewDefaultErrorListener()}
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser := p.NewQuintParser(tokens)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	tree := parser.Expr()
	if tokens.LA(1) != antlr.TokenEOF {
		listener.messages = append(listener.messages, "unexpected trailing input")
	}
	if len(listener.messages) > 0 {
		return Expr{}, fmt.Errorf("invalid expression: %s", strings.Join(listener.messages, "; "))
	}
	return expressionFromContext(tree, NewIDGenerator())
}
func expressionFromContext(ctx p.IExprContext, ids *IDGenerator) (Expr, error) {
	if c, ok := ctx.(*p.LiteralOrIdContext); ok {
		e := Expr{ID: ids.Next()}
		switch {
		case c.QualId() != nil:
			e.Kind = "name"
			e.Name = c.GetText()
		case c.INT() != nil:
			e.Kind = "int"
			n, err := ParseInteger(strings.ReplaceAll(c.GetText(), "_", ""))
			if err != nil {
				return Expr{}, err
			}
			e.Int = n
		case c.BOOL() != nil:
			e.Kind = "bool"
			e.Bool = c.GetText() == "true"
		case c.STRING() != nil:
			e.Kind = "str"
			e.Str = c.GetText()[1 : len(c.GetText())-1]
		}
		return e, nil
	}
	args := []Expr{}
	for _, child := range ctx.GetChildren() {
		if ex, ok := child.(p.IExprContext); ok {
			v, err := expressionFromContext(ex, ids)
			if err != nil {
				return Expr{}, err
			}
			args = append(args, v)
		}
	}
	appendArgs := func(list p.IArgListContext) error {
		if list == nil {
			return nil
		}
		for _, ctx := range list.AllExpr() {
			v, err := expressionFromContext(ctx, ids)
			if err != nil {
				return err
			}
			args = append(args, v)
		}
		return nil
	}
	opcode := ""
	switch c := ctx.(type) {
	case *p.ParenContext, *p.BracesContext:
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
	return Expr{ID: ids.Next(), Kind: "app", Opcode: opcode, Args: args}, nil
}

var tupleFieldPattern = regexp.MustCompile(`^_([1-9][0-9]?)$`)
