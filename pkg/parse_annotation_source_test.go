package quint

import "testing"

func TestParsedAnnotationSourceIDs(t *testing.T) {
	text := `module M {
 const n: int
 var xs: List[int]
 type Alias[a] = Set[a]
 type Choice[a] = Some(List[a]) | None
 def f(x: List[int]): Set[str] = Set()
 val g: int -> bool = x => true
}`
	ms, sources, err := ParseModulesWithSource("types.qnt", text)
	if err != nil {
		t.Fatal(err)
	}
	ds := ms[0].Declarations
	cases := []struct {
		typ  *Type
		want string
	}{
		{ds[0].TypeAnnotation, "int"}, {ds[1].TypeAnnotation, "List[int]"},
		{ds[2].Type, "Set[a]"}, {&ds[3].Type.Fields.Fields[0].Type, "List[a]"},
		{ds[6].Expr.Params[0].TypeAnnotation, "List[int]"},
		{ds[6].TypeAnnotation.Res, "Set[str]"}, {ds[7].TypeAnnotation, "int -> bool"},
	}
	for _, tc := range cases {
		if tc.typ.ID == nil {
			t.Fatalf("missing annotation ID for %s", tc.want)
		}
		if got := sourceExcerpt(t, text, sources, *tc.typ.ID); got != tc.want {
			t.Fatalf("span %q want %q", got, tc.want)
		}
		err := WalkType(tc.typ, func(node *Type) (bool, error) {
			if node.ID == nil {
				t.Fatalf("missing nested type ID %+v", node)
			}
			if _, ok := sources[*node.ID]; !ok {
				t.Fatal("missing nested type span")
			}
			return true, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	// The parameter annotation and operator argument refer to the same syntax.
	if *ds[6].Expr.Params[0].TypeAnnotation.ID != *ds[6].TypeAnnotation.Args[0].ID {
		t.Fatal("parameter annotation identity lost")
	}
}
func TestImportedAnnotationSource(t *testing.T) {
	r := &fixtureSourceResolver{sources: map[string]string{"main.qnt": `module Main { import A from "a" }`, "a.qnt": `module A { const n: List[int] }`}}
	ms, sources, err := LoadModulesWithSource(r, r.LookupPath(".", "main.qnt"))
	if err != nil {
		t.Fatal(err)
	}
	typ := ms[0].Declarations[0].TypeAnnotation
	if typ.ID == nil || typ.Elem.ID == nil || sources[*typ.ID].Source != "a.qnt" || sources[*typ.Elem.ID].Source != "a.qnt" {
		t.Fatal("imported annotation source")
	}
	plain, err := ParseType("List[int]")
	if err != nil || plain.ID != nil || plain.Elem.ID != nil {
		t.Fatal("signature parser should remain ID-free")
	}
}
