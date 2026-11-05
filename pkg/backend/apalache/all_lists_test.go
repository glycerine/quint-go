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

// TestFoldRightIsRejectedLikeUpstream pins upstream's rejection of foldr on the
// CHECK and TLA paths with the same diagnostic (verified against quint 0.33.1).
func TestFoldRightIsRejectedLikeUpstream(t *testing.T) {
	compiled := compileSource(t, `module M {
  var x: int
  action init = x' = List(1, 2, 3).foldr(0, (v, acc) => v - acc)
  action step = x' = x
  val inv = x >= -100
  val safe = true
  val wrong = false
}`)
	_, err := adaptFoldRight(compiled)
	if err == nil {
		t.Fatal("foldr was accepted; upstream rejects it")
	}
	want := "internal error: while parsing in Apalache:\n'Input was not a valid representation of the QuintIR: While converting operator application of defined operator 'foldr' to arguments List(QuintApp(6,List,List(QuintInt(3,1), QuintInt(4,2), QuintInt(5,3))), QuintInt(7,0), QuintLambda(13,List(QuintLambdaParameter(8,v_13), QuintLambdaParameter(9,acc_13)),def,QuintApp(12,isub,List(QuintName(10,v_13), QuintName(11,acc_13))))): Input was not a valid representation of the QuintIR: No entry found for id 14 in lookup table'\nPlease report an issue: https://github.com/informalsystems/quint/issues/new"
	if err.Error() != want {
		t.Fatalf("diagnostic mismatch:\n got: %q\nwant: %q", err.Error(), want)
	}
}
