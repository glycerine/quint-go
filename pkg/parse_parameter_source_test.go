package quint

import "testing"

func sourceExcerpt(t *testing.T, text string, sources SourceMap, id ID) string {
	t.Helper()
	file, err := NewSourceFile("", text)
	if err != nil {
		t.Fatal(err)
	}
	loc, ok := sources[id]
	if !ok || loc.End == nil {
		t.Fatalf("missing span for %v", id)
	}
	start, err := file.ByteOffset(loc.Start.Line, loc.Start.Col)
	if err != nil {
		t.Fatal(err)
	}
	end, err := file.ByteOffset(loc.End.Line, loc.End.Col)
	if err != nil {
		t.Fatal(err)
	}
	return text[start:end]
}
func TestParameterSourceSpans(t *testing.T) {
	for _, text := range []string{`(left, _) => left`, `((left,_)) => left`} {
		e, m, err := ParseExpressionWithSource("test", text)
		if err != nil {
			t.Fatal(err)
		}
		if text[1] == '(' {
			if got := sourceExcerpt(t, text, m, e.Body.Opdef.ID); got != "_" {
				t.Fatalf("hole span %q", got)
			}
			if got := sourceExcerpt(t, text, m, e.Body.Body.Opdef.ID); got != "left" {
				t.Fatalf("tuple binding span %q", got)
			}
		} else {
			for i, want := range []string{"left", "_"} {
				if got := sourceExcerpt(t, text, m, e.Params[i].ID); got != want {
					t.Fatalf("parameter %q want %q", got, want)
				}
			}
		}
	}
	text := `module M { def f(x: int, y: str): int = x }`
	ms, m, err := ParseModulesWithSource("test", text)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"x: int", "y: str"} {
		if got := sourceExcerpt(t, text, m, ms[0].Declarations[0].Expr.Params[i].ID); got != want {
			t.Fatalf("annotated parameter %q", got)
		}
	}
}
func TestGeneratedArmAndRecordSpans(t *testing.T) {
	text := `match x { A(v) => v | _ => 0 }`
	e, m, err := ParseExpressionWithSource("test", text)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"A(v) => v", "_ => 0"} {
		label, lambda := e.Args[1+2*i], e.Args[2+2*i]
		for _, id := range []ID{label.ID, lambda.ID, lambda.Params[0].ID} {
			if got := sourceExcerpt(t, text, m, id); got != want {
				t.Fatalf("arm span %q want %q", got, want)
			}
		}
	}
	text = `{ first: "😀", second: 2 }`
	e, m, err = ParseExpressionWithSource("test", text)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{`first: "😀"`, `second: 2`} {
		if got := sourceExcerpt(t, text, m, e.Args[i*2].ID); got != want {
			t.Fatalf("record label span %q", got)
		}
	}
}
