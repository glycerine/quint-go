package quint

import "testing"

func TestTypeSourceTree(t *testing.T) {
	for _, text := range []string{"Set[List[int]]", "(int,str) => bool", "int -> List[str]", "{x: Set[int], y: a | r}", "Foo[int, List[a]]", "(int, str)", "()", "(int)"} {
		typ, sources, err := ParseTypeWithSource("type", text)
		if err != nil {
			t.Fatal(err)
		}
		plain, err := ParseType(text)
		if err != nil || TypeString(typ) != TypeString(plain) {
			t.Fatal("source parser changed type")
		}
		count := 0
		err = WalkType(&typ, func(node *Type) (bool, error) {
			if node.ID == nil {
				t.Fatalf("%s: missing type ID %+v", text, node)
			}
			loc, ok := sources[*node.ID]
			if !ok || loc.Source != "type" || loc.End == nil {
				t.Fatal("missing type span")
			}
			count++
			return true, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if count != len(sources) {
			t.Fatalf("%s: %d nodes vs %d spans", text, count, len(sources))
		}
	}
}
func TestTypeExactSourceSpans(t *testing.T) {
	text := "{x: List[int], y: Set[str]}"
	typ, m, err := ParseTypeWithSource("t", text)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		typ  Type
		want string
	}{{typ, text}, {typ.Fields.Fields[0].Type, "List[int]"}, {*typ.Fields.Fields[0].Type.Elem, "int"}, {typ.Fields.Fields[1].Type, "Set[str]"}, {*typ.Fields.Fields[1].Type.Elem, "str"}} {
		if got := sourceExcerpt(t, text, m, *tc.typ.ID); got != tc.want {
			t.Fatalf("span %q want %q", got, tc.want)
		}
	}
	typ, m, err = ParseTypeWithSource("t", "(int)")
	if err != nil || sourceExcerpt(t, "(int)", m, *typ.ID) != "int" {
		t.Fatal("parentheses changed type span")
	}
	for _, text := range []string{"List[", "Map[int,str]", string([]byte{0xff})} {
		if _, m, err := ParseTypeWithSource("bad", text); err == nil || m != nil {
			t.Fatalf("accepted %q", text)
		}
	}
}

func TestParsedUnitTypeRow(t *testing.T) {
	typ, err := ParseType("()")
	if err != nil {
		t.Fatal(err)
	}
	if typ.Kind != "tup" || typ.Fields.Kind != "empty" {
		t.Fatalf("parsed unit %+v", typ)
	}
	synthesized := UnitType()
	if synthesized.Fields.Kind != "row" {
		t.Fatal("synthesized unit representation changed")
	}
}
