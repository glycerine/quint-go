package quint

import (
	"github.com/antlr4-go/antlr/v4"
	p "github.com/glycerine/quint-go/pkg/internal/quintparser"
)

// SLL avoids full-context prediction for valid modules. ANTLR guarantees that
// an error-free SLL parse produces the same tree as LL. On any lexical, grammar
// action, or prediction error, discard the attempt and repeat the original LL
// parse with fresh state and the caller's diagnostic listener.
func parseModuleTree(text string, listener *effectSyntaxErrors) p.IModulesContext {
	if tree, ok := parseModuleTreeSLL(text); ok {
		return tree
	}
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	parser := p.NewQuintParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	return parser.Modules()
}

func parseModuleTreeSLL(text string) (tree p.IModulesContext, ok bool) {
	defer func() {
		if failure := recover(); failure != nil {
			if _, canceled := failure.(modulePredictionCanceled); !canceled {
				panic(failure)
			}
			tree, ok = nil, false
		}
	}()
	// This listener has no onIssue callback. A rejected speculation must not
	// allocate source IDs or publish diagnostics before the LL retry.
	listener := newSyntaxErrors(text)
	lexer := p.NewQuintLexer(antlr.NewInputStream(text))
	lexer.RemoveErrorListeners()
	lexer.AddErrorListener(listener)
	parser := p.NewQuintParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
	parser.RemoveErrorListeners()
	parser.AddErrorListener(listener)
	parser.GetInterpreter().SetPredictionMode(antlr.PredictionModeSLL)
	parser.SetErrorHandler(&modulePredictionBail{antlr.NewDefaultErrorStrategy()})
	tree = parser.Modules()
	return tree, len(listener.issues) == 0 && !parser.HasError()
}

type modulePredictionCanceled struct{}

type modulePredictionBail struct{ *antlr.DefaultErrorStrategy }

// Generated Go rules can clear the error set by ANTLR's stock BailErrorStrategy.
// A private panic exits the entire speculative parse before recovery can alter
// its tree; only parseModuleTreeSLL catches it.
func (*modulePredictionBail) Recover(antlr.Parser, antlr.RecognitionException) {
	panic(modulePredictionCanceled{})
}

func (*modulePredictionBail) RecoverInline(antlr.Parser) antlr.Token {
	panic(modulePredictionCanceled{})
}

func (*modulePredictionBail) Sync(antlr.Parser) {}
