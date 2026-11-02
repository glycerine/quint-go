package quint

import (
	"reflect"
	"testing"
)

func TestSimplifyRows(t *testing.T) {
	tail := Row{Kind: "var", Name: "r"}
	nested := Row{Kind: "row", Fields: []RowField{{Name: "a", Type: Type{Kind: "int"}}}, Other: &Row{Kind: "row", Fields: []RowField{{Name: "b", Type: Type{Kind: "bool"}}}, Other: &Row{Kind: "row", Other: &tail}}}
	got := SimplifyRow(nested)
	if !reflect.DeepEqual(RowFieldNames(&got), []string{"a", "b"}) || !reflect.DeepEqual(*got.Other, tail) {
		t.Fatalf("flattened: %+v", got)
	}
	if !reflect.DeepEqual(SimplifyRow(got), got) {
		t.Fatal("not idempotent")
	}
	got.Fields[0].Name = "changed"
	got.Other.Name = "changed"
	if nested.Fields[0].Name != "a" || tail.Name != "r" {
		t.Fatal("mutated original")
	}
	for _, r := range []Row{{Kind: "empty"}, tail} {
		if !reflect.DeepEqual(SimplifyRow(r), r) {
			t.Fatal("changed terminal row")
		}
	}
	if got := SimplifyRow(Row{Kind: "row", Other: &Row{Kind: "empty"}}); got.Kind != "empty" {
		t.Fatal("empty row not eliminated")
	}
}
func TestSimplifySchemeCopiesAllChildren(t *testing.T) {
	id, _ := ParseID("1")
	leaf := Type{ID: &id, Kind: "var", Name: "a"}
	typ := Type{Kind: "app", Ctor: &Type{Kind: "const", Name: "T"}, Args: []Type{{Kind: "fun", Arg: &Type{Kind: "set", Elem: &leaf}, Res: &Type{Kind: "list", Elem: &leaf}}, {Kind: "rec", Fields: &Row{Kind: "row", Other: &Row{Kind: "var", Name: "r"}}}}}
	s := TypeScheme{Type: typ, TypeVariables: []string{"a"}, RowVariables: []string{"r"}}
	got := SimplifyScheme(s)
	if got.Type.Args[1].Fields.Kind != "var" {
		t.Fatal("nested row not simplified")
	}
	got.Type.Args[0].Arg.Elem.Name = "b"
	got.Type.Args[0].Res.Elem.ID = nil
	got.Type.Ctor.Name = "U"
	got.TypeVariables[0] = "b"
	got.RowVariables[0] = "s"
	if leaf.Name != "a" || leaf.ID == nil || typ.Ctor.Name != "T" || s.TypeVariables[0] != "a" || s.RowVariables[0] != "r" {
		t.Fatal("simplification aliased input")
	}
}
func TestFreshNames(t *testing.T) {
	var g FreshNames
	for _, c := range [][2]string{{"a", "a0"}, {"b", "b0"}, {"a", "a1"}, {"", "0"}, {"a", "a2"}, {"", "1"}} {
		if got := g.Next(c[0]); got != c[1] {
			t.Fatalf("got %s want %s", got, c[1])
		}
	}
	var other FreshNames
	if other.Next("a") != "a0" {
		t.Fatal("shared counters")
	}
}
