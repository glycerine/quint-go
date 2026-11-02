package quint

import (
	"reflect"
	"testing"
)

func TestConstraintSubstitution(t *testing.T) {
	id, _ := ParseID("123456789012345678901234567890")
	a := Type{Kind: "var", Name: "a"}
	root := Constraint{Kind: "conjunction", SourceID: id, Constraints: []Constraint{
		{Kind: "eq", SourceID: id, Types: [2]Type{a, {Kind: "set", Elem: &a}}},
		{Kind: "conjunction", Constraints: []Constraint{{Kind: "isDefined", SourceID: id, Type: a}, {Kind: "empty"}}},
	}}
	got, err := ApplyConstraintSubstitution([]Substitution{{Kind: "type", Name: "a", Type: Type{Kind: "int"}}}, root)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceID != id || got.Constraints[0].SourceID != id || got.Constraints[1].Constraints[0].SourceID != id {
		t.Fatal("lost source identity")
	}
	if got.Constraints[0].Types[0].Kind != "int" || got.Constraints[0].Types[1].Elem.Kind != "int" || got.Constraints[1].Constraints[0].Type.Kind != "int" {
		t.Fatalf("missed nested substitution: %+v", got)
	}
	if root.Constraints[0].Types[0].Kind != "var" {
		t.Fatal("input changed")
	}
	got.Constraints[1].Constraints[0].Kind = "changed"
	if root.Constraints[1].Constraints[0].Kind != "isDefined" {
		t.Fatal("aliased constraints")
	}
	same, err := ApplyConstraintSubstitution(nil, root)
	if err != nil || !reflect.DeepEqual(same, root) {
		t.Fatal("identity substitution changed constraint")
	}
}
func TestConstraintErrors(t *testing.T) {
	bad := Type{Kind: "unknown"}
	for _, c := range []Constraint{{Kind: "unknown"}, {Kind: "eq", Types: [2]Type{bad, {Kind: "int"}}}, {Kind: "eq", Types: [2]Type{{Kind: "int"}, bad}}, {Kind: "isDefined", Type: bad}, {Kind: "conjunction", Constraints: []Constraint{{Kind: "unknown"}}}} {
		if _, err := ApplyConstraintSubstitution(nil, c); err == nil {
			t.Errorf("accepted %+v", c)
		}
	}
	for i, kind := range []string{"empty", "eq", "conjunction", "isDefined"} {
		rank, err := ConstraintRank(kind)
		if err != nil || rank != i {
			t.Fatalf("rank %s: %d %v", kind, rank, err)
		}
	}
	if _, err := ConstraintRank("unknown"); err == nil {
		t.Fatal("accepted unknown rank")
	}
}
