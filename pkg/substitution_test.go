package quint

import (
	"reflect"
	"testing"
)

func TestApplySubstitution(t *testing.T) {
	subs := []Substitution{{Kind: "type", Name: "a", Type: Type{Kind: "var", Name: "b"}}, {Kind: "type", Name: "b", Type: Type{Kind: "int"}}, {Kind: "row", Name: "r", Row: Row{Kind: "row", Fields: []RowField{{Name: "y", Type: Type{Kind: "var", Name: "a"}}}, Other: &Row{Kind: "empty"}}}}
	input := Type{Kind: "oper", Args: []Type{{Kind: "list", Elem: &Type{Kind: "var", Name: "a"}}}, Res: &Type{Kind: "rec", Fields: &Row{Kind: "var", Name: "r"}}}
	got, err := ApplySubstitution(subs, input)
	if err != nil {
		t.Fatal(err)
	}
	if got.Args[0].Elem.Kind != "int" || got.Res.Fields.Fields[0].Type.Kind != "int" {
		t.Fatalf("unresolved chain: %+v", got)
	}
	if input.Args[0].Elem.Kind != "var" || subs[2].Row.Fields[0].Type.Kind != "var" {
		t.Fatal("input mutated")
	}
	got.Res.Fields.Fields[0].Name = "changed"
	if subs[2].Row.Fields[0].Name != "y" {
		t.Fatal("aliased replacement")
	}
	same, err := ApplySubstitution(nil, input)
	if err != nil || !reflect.DeepEqual(same, input) {
		t.Fatal("empty substitution changed type")
	}
}
func TestSubstitutionCyclesAndOrder(t *testing.T) {
	variable := Type{Kind: "var", Name: "a"}
	for _, subs := range [][]Substitution{
		{{Kind: "type", Name: "a", Type: Type{Kind: "set", Elem: &variable}}},
		{{Kind: "type", Name: "a", Type: Type{Kind: "var", Name: "b"}}, {Kind: "type", Name: "b", Type: variable}},
	} {
		if _, err := ApplySubstitution(subs, variable); err == nil {
			t.Fatal("accepted cycle")
		}
	}
	got, err := ApplySubstitution([]Substitution{{Kind: "type", Name: "a", Type: variable}}, variable)
	if err != nil || !reflect.DeepEqual(got, variable) {
		t.Fatal("identity substitution failed")
	}
	got, err = ApplySubstitution([]Substitution{{Kind: "row", Name: "a", Row: Row{Kind: "empty"}}, {Kind: "type", Name: "a", Type: Type{Kind: "int"}}}, variable)
	if err != nil || got.Kind != "var" {
		t.Fatal("changed upstream first-name lookup semantics")
	}
	r := Type{Kind: "rec", Fields: &Row{Kind: "var", Name: "r"}}
	if _, err := ApplySubstitution([]Substitution{{Kind: "row", Name: "r", Row: Row{Kind: "row", Other: &Row{Kind: "var", Name: "r"}}}}, r); err == nil {
		t.Fatal("accepted row cycle")
	}
	if _, err := ApplySubstitution(nil, Type{Kind: "bad"}); err == nil {
		t.Fatal("accepted malformed input")
	}
}
