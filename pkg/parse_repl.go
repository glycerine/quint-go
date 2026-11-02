package quint

import (
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

// ExpressionOrDeclaration is a syntax-only REPL result. Kind is "declaration",
// "expr", or "none"; only the corresponding payload is populated.
type ExpressionOrDeclaration struct {
	Kind         string
	Declarations []Declaration
	Expr         *Expr
}

// ParseExpressionOrDeclaration parses the upstream declarationOrExpr rule.
// Reusing ids across calls gives a REPL session unique IDs; nil starts a new
// sequence. Failed conversions may consume IDs. Errors return no partial IR.
func ParseExpressionOrDeclaration(text string, ids *IDGenerator) (ExpressionOrDeclaration, error) {
	if ids == nil {
		ids = NewIDGenerator()
	}
	return parseREPLWithIDs(text, ids)
}
func parseREPLWithIDs(text string, ids parserIDAllocator) (ExpressionOrDeclaration, error) {
	listener := newSyntaxErrors(text)
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	parser := p.NewQuintParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	tree := parser.DeclarationOrExpr()
	if len(listener.issues) > 0 {
		return ExpressionOrDeclaration{}, listener.err("REPL input")
	}
	if tree.Declaration() != nil {
		ds, err := declarationsFromContext(tree.Declaration(), ids)
		if err != nil {
			return ExpressionOrDeclaration{}, err
		}
		return ExpressionOrDeclaration{Kind: "declaration", Declarations: ds}, nil
	}
	if tree.Expr() != nil {
		expr, err := expressionFromContext(tree.Expr(), ids)
		if err != nil {
			return ExpressionOrDeclaration{}, err
		}
		return ExpressionOrDeclaration{Kind: "expr", Expr: &expr}, nil
	}
	return ExpressionOrDeclaration{Kind: "none"}, nil
}
