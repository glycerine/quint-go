package apalache

import (
	"strings"
	"testing"
)

// TestAllListsUpToIsRejectedLikeUpstream pins upstream's rejection of allListsUpTo:
// the Apalache QuintIR parser fails with the same three-line diagnostic and the
// same lookup ID upstream reports for this spec (verified against quint 0.33.1).
func TestAllListsUpToIsRejectedLikeUpstream(t *testing.T) {
	compiled := compileSource(t, `module M {
  var x: Set[List[int]]
  action init = x' = allListsUpTo(Set(1, 2), 2)
  action step = x' = x
  val inv = x.size() >= 0
  val safe = true
  val wrong = false
}`)
	_, err := adaptAllListsUpTo(compiled)
	if err == nil {
		t.Fatal("allListsUpTo was accepted; upstream rejects it")
	}
	want := "internal error: while parsing in Apalache:\n'Input was not a valid representation of the QuintIR: While converting operator application of defined operator 'allListsUpTo' to arguments List(QuintApp(7,Set,List(QuintInt(5,1), QuintInt(6,2))), QuintInt(8,2)): Input was not a valid representation of the QuintIR: No entry found for id 9 in lookup table'\nPlease report an issue: https://github.com/informalsystems/quint/issues/new"
	if err.Error() != want {
		t.Fatalf("diagnostic mismatch:\n got: %q\nwant: %q", err.Error(), want)
	}
	if !strings.HasPrefix(err.Error(), "internal error: while parsing in Apalache:") {
		t.Fatal("diagnostic must start with the upstream header")
	}
}
