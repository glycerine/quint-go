package quint

import (
	"reflect"
	"testing"
)

func TestExpressionSourceMap(t *testing.T) {
	text := `"😀" + 123`
	expr, sources, err := ParseExpressionWithSource("test.qnt", text)
	if err != nil {
		t.Fatal(err)
	}
	want, err := ParseExpression(text)
	if err != nil || !reflect.DeepEqual(expr, want) {
		t.Fatal("source tracking changed IR")
	}
	for _, tc := range []struct {
		id         ID
		start, end int
	}{{expr.ID, 0, 10}, {expr.Args[0].ID, 0, 4}, {expr.Args[1].ID, 7, 10}} {
		loc, ok := sources[tc.id]
		if !ok || loc.Source != "test.qnt" || loc.Start.Index != tc.start || loc.End == nil || loc.End.Index != tc.end {
			t.Fatalf("location %s: %+v", tc.id, loc)
		}
	}
}
func TestSourceMapGeneratedAndLocalIDs(t *testing.T) {
	for _, text := range []string{`((a,b)) => a+b`, `val (a,b) = pair; a+b`, `match x { A(v) => v | _ => 0 }`, `q::debug(x)`, `val f(x) = x+1; f(2)`} {
		expr, sources, err := ParseExpressionWithSource("example", text)
		if err != nil {
			t.Fatal(err)
		}
		check := func(id ID) {
			t.Helper()
			loc, ok := sources[id]
			if !ok || loc.Source != "example" || loc.End == nil || loc.Start.Index > loc.End.Index {
				t.Fatalf("%s: missing/invalid ID %v", text, id)
			}
		}
		err = WalkExpr(&expr, func(e *Expr) (bool, error) {
			check(e.ID)
			for _, p := range e.Params {
				check(p.ID)
			}
			if e.Opdef != nil {
				check(e.Opdef.ID)
			}
			return true, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
func TestExpressionSourceFailure(t *testing.T) {
	for _, text := range []string{"1 +", string([]byte{0xff})} {
		if _, sources, err := ParseExpressionWithSource("bad", text); err == nil || sources != nil {
			t.Fatalf("accepted %q", text)
		}
	}
	e, m, err := ParseExpressionWithSource("p", "(1)")
	if err != nil || m[e.ID].Start.Index != 1 || m[e.ID].End.Index != 2 {
		t.Fatal("parentheses overwrote literal span")
	}
}
