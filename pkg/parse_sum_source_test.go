package quint

import "testing"

func TestSumTypeIdentitiesAndSpans(t *testing.T) {
	text := `module M { type Result[a] = Some(List[a]) | None }`
	ms, m, err := ParseModulesWithSource("sum", text)
	if err != nil {
		t.Fatal(err)
	}
	ds := ms[0].Declarations
	sum := ds[0]
	if sum.Type.ID == nil || *sum.Type.ID != sum.ID {
		t.Fatal("sum type must share declaration identity")
	}
	result := ds[1].TypeAnnotation.Res
	if result.ID == nil || *result.ID != sum.ID || result.Ctor.ID == nil || result.Args[0].ID == nil {
		t.Fatal("generic result identity")
	}
	for _, typ := range []*Type{result, result.Ctor, &result.Args[0]} {
		if sourceExcerpt(t, text, m, *typ.ID) != "type Result[a] = Some(List[a]) | None" {
			t.Fatal("generated result span")
		}
	}
	unit := sum.Type.Fields.Fields[1].Type
	if unit.ID == nil || sourceExcerpt(t, text, m, *unit.ID) != "None" {
		t.Fatal("unit payload type span")
	}
	lambda := ds[1].Expr
	if sourceExcerpt(t, text, m, lambda.Params[0].ID) != "List[a]" {
		t.Fatal("payload parameter span")
	}
	if sourceExcerpt(t, text, m, lambda.Body.Args[1].ID) != "Some" {
		t.Fatal("payload reference span")
	}
	// Generated operator type wrappers intentionally have no ID upstream.
	if ds[1].TypeAnnotation.ID != nil {
		t.Fatal("unexpected synthesized operator type ID")
	}
}
func TestSumAliasAndMonomorphicResultSpans(t *testing.T) {
	for _, text := range []string{`module M { type T = A }`, `module M { type T = | A }`} {
		ms, m, err := ParseModulesWithSource("sum", text)
		if err != nil {
			t.Fatal(err)
		}
		ds := ms[0].Declarations
		if len(ds) == 1 {
			if ds[0].Type.ID == nil || sourceExcerpt(t, text, m, *ds[0].Type.ID) != "A" {
				t.Fatal("alias target span")
			}
		} else {
			typ := ds[1].TypeAnnotation
			if typ.ID == nil || typ.Kind != "const" || sourceExcerpt(t, text, m, *typ.ID) != "type T = | A" {
				t.Fatal("monomorphic result span")
			}
		}
	}
}
