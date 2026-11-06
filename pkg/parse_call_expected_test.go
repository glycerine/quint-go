package quint

import (
	"strings"
	"testing"
)

// TestCallParenExpectedTokensMatchUpstream pins upstream's expected-token text for a
// '(' that follows a complete expression at the top level, such as Set(1 2). The
// inputs and text were captured from the repository's compiled TypeScript parser (F-02).
func TestCallParenExpectedTokensMatchUpstream(t *testing.T) {
	const want = "mismatched input '(' expecting {<EOF>, '[', '.', '->', '^', '::', 'and', 'or', 'iff', 'implies', 'leadsTo', '+', '-', '*', '/', '%', '>', '<', '>=', '<=', '!=', '==', '='}"
	for _, input := range []string{`Set(1 2)`, `foo(1 2)`, `f(a b)`, `List(1 2)`, `Set(1, 2 3)`, `f(1 2 3)`} {
		_, err := ParseExpressionOrDeclaration(input, NewIDGenerator())
		if err == nil {
			t.Fatalf("%s accepted", input)
		}
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: expected-token text differs from upstream:\n got: %q", input, err.Error())
		}
	}
}
