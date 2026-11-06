package quint

import (
	"strings"
	"testing"
)

// TestREPLOmitsCaretOnEmptyEndOfInputLine pins upstream's REPL block for an error at
// the end of an input whose last line is empty (F-04 in AUDIT.md).
func TestREPLOmitsCaretOnEmptyEndOfInputLine(t *testing.T) {
	block := "\n Error [QNT000]: no viable alternative\n\n  at input:1:1\n  \n  ^\n"
	want := "\n Error [QNT000]: no viable alternative\n\n  at input:1:1\n  \n  \n"
	if got := omitEmptyLineCaret(block); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
}

// TestREPLKeepsCaretOnNonEmptyLine ensures the rule only touches empty excerpts.
func TestREPLKeepsCaretOnNonEmptyLine(t *testing.T) {
	block := "\n Error [QNT000]: mismatched input\n\n  at input:1:9\n  val x = \"abc\n          ^\n"
	if got := omitEmptyLineCaret(block); got != block {
		t.Fatalf("non-empty excerpt changed:\n%q", got)
	}
	if strings.Contains(omitEmptyLineCaret("plain text"), "^") {
		t.Fatal("plain text gained a caret")
	}
}
