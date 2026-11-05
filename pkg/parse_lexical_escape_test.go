package quint

import (
	"strings"
	"testing"
)

// TestREPLLexicalErrorEscapesLineBreaks pins upstream's text for an unterminated
// string in REPL input: the newline in the token excerpt is the two characters
// backslash and n, not a raw line break (verified against quint 0.33.1).
func TestREPLLexicalErrorEscapesLineBreaks(t *testing.T) {
	_, err := ParseExpressionOrDeclaration("val x = \"abc\n", NewIDGenerator())
	if err == nil {
		t.Fatal("unterminated string accepted")
	}
	want := `token recognition error at: '"abc\n'`
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("lexical excerpt not escaped:\n got: %q\nwant substring: %q", err.Error(), want)
	}
	if strings.Contains(err.Error(), "'\"abc\n") {
		t.Fatal("raw line break leaked into the lexical excerpt")
	}
}

// TestEscapeLexicalExcerptIsIdempotent guards the double application on the
// REPL and recovering paths.
func TestEscapeLexicalExcerptIsIdempotent(t *testing.T) {
	once := escapeLexicalExcerpt("token recognition error at: '\"a\tb\n'")
	twice := escapeLexicalExcerpt(once)
	if once != twice || once != `token recognition error at: '"a\tb\n'` {
		t.Fatalf("escape not idempotent: once=%q twice=%q", once, twice)
	}
	if got := escapeLexicalExcerpt("mismatched input"); got != "mismatched input" {
		t.Fatalf("non-lexical message changed: %q", got)
	}
}
