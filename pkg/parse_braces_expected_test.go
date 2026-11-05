package quint

import (
	"strings"
	"testing"
)

// TestBraceKeySeparatorExpectedTokensMatchUpstream pins upstream's expected-token
// text for a ':' that follows an expression inside braces. The inputs and text were
// captured from quint 0.33.1 REPL output (F-02 in AUDIT.md).
func TestBraceKeySeparatorExpectedTokensMatchUpstream(t *testing.T) {
	const want = "mismatched input ':' expecting {'}', '[', '.', '->', '^', 'and', 'or', 'iff', 'implies', 'leadsTo', '+', '-', '*', '/', '%', '>', '<', '>=', '<=', '!=', '==', '='}"
	for _, input := range []string{`{1: 2}`, `{"a": 1 "b": 2}`, `{"k": 1 "j": 2}`, `{"weird key": 1, b: 2}`} {
		_, err := ParseExpressionOrDeclaration(input, NewIDGenerator())
		if err == nil {
			t.Fatalf("%s accepted", input)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: expected-token text differs from upstream:\n got: %q", input, err.Error())
		}
	}
}

// TestBraceAfterCompleteRecordKeepsOwnExpectedText guards the rewrite: an
// unrelated brace error keeps the Go expected-token text.
func TestBraceAfterCompleteRecordKeepsOwnExpectedText(t *testing.T) {
	_, err := ParseExpressionOrDeclaration(`{a: 1 b: 2}`, NewIDGenerator())
	if err == nil || !strings.Contains(err.Error(), "mismatched input 'b' expecting {'}', ','}") {
		t.Fatalf("unexpected message: %v", err)
	}
}
