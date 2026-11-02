package quintparser

import (
	"github.com/antlr4-go/antlr/v4"
	"strings"
	"testing"
)

type syntaxErrors struct {
	*antlr.DefaultErrorListener
	messages []string
}

func (e *syntaxErrors) SyntaxError(_ antlr.Recognizer, _ interface{}, _, _ int, msg string, _ antlr.RecognitionException) {
	e.messages = append(e.messages, msg)
}
func TestQuintGrammar(t *testing.T) {
	cases := []struct{ text, code string }{
		{"module M { pure val x = 1 + 2 * 3 }", ""},
		{"module M { var x: int action init = x' = 0 action step = x' = x + 1 }", ""},
		{"module M { type T[a] = A(a) | B pure def f(x) = match x { A(v) => v | B => 0 } }", ""},
		{"module M { import X.* from \"x\" pure val r = { a: 1, b: true } }", ""},
		{"module M { val x = 1 = 2 }", "QNT006"},
		{"module M { var x: Foo(int) }", "QNT009"},
		{"module M { val x = { a::b: 1 } }", "QNT008"},
		{"module M { var x }", "error"},
	}
	for _, c := range cases {
		listener := &syntaxErrors{DefaultErrorListener: antlr.NewDefaultErrorListener()}
		lexer := NewQuintLexer(antlr.NewInputStream(c.text))
		lexer.RemoveErrorListeners()
		lexer.AddErrorListener(listener)
		parser := NewQuintParser(antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel))
		parser.RemoveErrorListeners()
		parser.AddErrorListener(listener)
		parser.Modules()
		if c.code == "" && len(listener.messages) > 0 {
			t.Errorf("%s: %v", c.text, listener.messages)
		}
		if c.code != "" && (len(listener.messages) == 0 || (c.code != "error" && !strings.Contains(strings.Join(listener.messages, " "), c.code))) {
			t.Errorf("%s: wanted %s got %v", c.text, c.code, listener.messages)
		}
	}
}
