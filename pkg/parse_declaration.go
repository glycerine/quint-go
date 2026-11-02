package quint

import (
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

// ParseDeclaration parses one complete source declaration, optionally preceded
// by documentation comments. Sum types and destructuring can expand into several
// IR declarations. It performs no name resolution or semantic analysis.
func ParseDeclaration(text string) ([]Declaration, error) {
	listener := newSyntaxErrors(text)
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	parser := p.NewQuintParser(tokens)
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	docs := []string{}
	for tokens.LA(1) == p.QuintParserDOCCOMMENT {
		docs = append(docs, tokens.LT(1).GetText())
		tokens.Consume()
	}
	tree := parser.Declaration()
	if tokens.LA(1) != antlr.TokenEOF {
		listener.trailing(tokens.LT(1))
	}
	if len(listener.issues) > 0 {
		return nil, listener.err("declaration")
	}
	ds, err := declarationsFromContext(tree, NewIDGenerator())
	if err != nil {
		return nil, err
	}
	ds[len(ds)-1].Doc = joinedDocs(docs)
	return ds, nil
}

func declarationsFromContext(ctx p.IDeclarationContext, ids parserIDAllocator) ([]Declaration, error) {
	if oper, ok := ctx.(*p.OperContext); ok {
		defs, err := destructuredOperators(oper.OperDef(), ids)
		if err != nil {
			return nil, err
		}
		if defs != nil {
			ds := make([]Declaration, len(defs))
			for i, op := range defs {
				ds[i] = Declaration{ID: op.ID, Kind: op.Kind, Name: op.Name, Qualifier: op.Qualifier, Expr: op.Expr}
			}
			return ds, nil
		}
	}
	if td, ok := ctx.(*p.TypeDefsContext); ok {
		if sum, ok := td.TypeDef().(*p.TypeSumDefContext); ok {
			return sumDeclarations(sum, ids)
		}
	}
	d, err := declarationFromContext(ctx, ids)
	if err != nil {
		return nil, err
	}
	return []Declaration{d}, nil
}
