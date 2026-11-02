package effectparser

import (
	"github.com/antlr4-go/antlr/v4"
	"testing"
)

type errors struct {
	*antlr.DefaultErrorListener
	count int
}

func (e *errors) SyntaxError(_ antlr.Recognizer, _ interface{}, _, _ int, _ string, _ antlr.RecognitionException) {
	e.count++
}
func TestEffectGrammar(t *testing.T) {
	for _, c := range []struct {
		input string
		valid bool
	}{{"Pure", true}, {"(Read[v], Update['x']) => Read[v] & Update['x']", true}, {"Temporal['x', v]", true}, {"() => Pure", true}, {"Read[", false}, {"Update[x", false}, {"Pure trailing", false}, {"Read[@]", false}} {
		listener := &errors{DefaultErrorListener: antlr.NewDefaultErrorListener()}
		lexer := NewEffectLexer(antlr.NewInputStream(c.input))
		lexer.RemoveErrorListeners()
		lexer.AddErrorListener(listener)
		tokens := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
		parser := NewEffectParser(tokens)
		parser.RemoveErrorListeners()
		parser.AddErrorListener(listener)
		parser.Effect()
		valid := listener.count == 0 && tokens.LA(1) == antlr.TokenEOF
		if valid != c.valid {
			t.Errorf("%q: valid=%v errors=%d", c.input, valid, listener.count)
		}
	}
}
