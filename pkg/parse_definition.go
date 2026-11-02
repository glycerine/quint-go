package quint

import (
	"fmt"
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

func ParseOperatorDefinition(text string) (OperatorDef, error) {
	listener := newSyntaxErrors(text)
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser := p.NewQuintParser(tokens)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	tree := parser.OperDef()
	if tokens.LA(1) != antlr.TokenEOF {
		listener.trailing(tokens.LT(1))
	}
	if len(listener.issues) > 0 {
		return OperatorDef{}, listener.err("definition")
	}
	return operatorFromContext(tree, NewIDGenerator())
}
func operatorFromContext(ctx p.IOperDefContext, ids parserIDAllocator) (OperatorDef, error) {
	defer parserScope(ids, ctx)()
	var name string
	var nameContext p.INormalCallNameContext
	var qualifier Qualifier
	var expr p.IExprContext
	var annotation *Type
	params := []Parameter{}
	switch c := ctx.(type) {
	case *p.DeprecatedOperDefContext:
		nameContext = c.NormalCallName()
		name = nameContext.GetText()
		qualifier = Qualifier(c.Qualifier().GetText())
		expr = c.Expr()
		for _, param := range c.AllParameter() {
			params = append(params, Parameter{ID: parserIDAt(ids, param), Name: param.GetText()})
		}
		if c.Type_() != nil {
			t, err := typeFromContextWithIDs(c.Type_(), ids)
			if err != nil {
				return OperatorDef{}, err
			}
			annotation = &t
		}
	case *p.AnnotatedOperDefContext:
		nameContext = c.NormalCallName()
		name = nameContext.GetText()
		qualifier = Qualifier(c.Qualifier().GetText())
		expr = c.Expr()
		types := []Type{}
		for _, param := range c.AllAnnotatedParameter() {
			t, err := typeFromContextWithIDs(param.Type_(), ids)
			if err != nil {
				return OperatorDef{}, err
			}
			params = append(params, Parameter{ID: parserIDAt(ids, param), Name: param.IdentOrHole().GetText(), TypeAnnotation: &t})
			types = append(types, t)
		}
		result, err := typeFromContextWithIDs(c.Type_(), ids)
		if err != nil {
			return OperatorDef{}, err
		}
		annotation = &Type{Kind: "oper", Args: types, Res: &result}
	default:
		return OperatorDef{}, fmt.Errorf("definition conversion not implemented for %T", ctx)
	}
	var body Expr
	if expr == nil {
		body = Expr{ID: ids.Next(), Kind: "bool", Bool: true}
	} else {
		v, err := expressionFromContext(expr, ids)
		if err != nil {
			return OperatorDef{}, err
		}
		body = v
	}
	if len(params) > 0 {
		inner := body
		body = Expr{ID: ids.Next(), Kind: "lambda", Params: params, Qualifier: qualifier, Body: &inner}
	}
	return OperatorDef{ID: parserIDAt(ids, nameContext), Kind: "def", Name: name, Qualifier: qualifier, Expr: &body, TypeAnnotation: annotation}, nil
}
