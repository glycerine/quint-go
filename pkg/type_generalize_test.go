package quint

import (
	"reflect"
	"testing"
)

func TestScopeFreeVariableOrdering(t *testing.T) {
	parse := func(text string) Type {
		v, e := ParseType(text)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	scope := []TypeScheme{
		{Type: parse("(a,b,{x:c|r})"), TypeVariables: []string{"c"}},
		{Type: parse("(b,d,{x:a|s})"), RowVariables: []string{"s"}},
	}
	free, err := FreeVariablesInScope(scope)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(free.Types, []string{"b", "d", "a"}) || !reflect.DeepEqual(free.Rows, []string{"r"}) {
		t.Fatal(free)
	}
}
func TestFreeVariableExpansion(t *testing.T) {
	typ, _ := ParseType("(b,{x:c|r})")
	nested, _ := ParseType("List[d]")
	row := Row{Kind: "row", Fields: []RowField{{Name: "field", Type: Type{Kind: "var", Name: "ignored"}}}, Other: &Row{Kind: "var", Name: "s"}}
	before := TypeVariables{Types: []string{"a"}, Rows: []string{}}
	out, err := ExpandFreeVariables(before, []Substitution{
		{Kind: "type", Name: "a", Type: typ},
		{Kind: "type", Name: "b", Type: nested},
		{Kind: "row", Name: "r", Row: row},
		{Kind: "type", Name: "unrelated", Type: Type{Kind: "var", Name: "unused"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.Types, []string{"a", "b", "c", "d"}) || !reflect.DeepEqual(out.Rows, []string{"r", "s"}) {
		t.Fatal(out)
	}
	if !reflect.DeepEqual(before.Types, []string{"a"}) || len(before.Rows) != 0 {
		t.Fatal("input mutated")
	}
	// Row field type variables are deliberately not added by row bindings.
}
func TestGeneralizeType(t *testing.T) {
	typ, _ := ParseType("(a,b,{x:c|r})")
	after := TypeVariables{Types: []string{"b", "a", "c"}, Rows: []string{"r", "s"}}
	before := TypeVariables{Types: []string{"a"}, Rows: []string{"s"}}
	scheme, err := GeneralizeType(typ, after, before)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scheme.TypeVariables, []string{"b", "c"}) || !reflect.DeepEqual(scheme.RowVariables, []string{"r"}) {
		t.Fatal(scheme)
	}
	scheme.Type.Fields.Fields[0].Type.Name = "changed"
	if typ.Fields.Fields[0].Type.Name != "a" {
		t.Fatal("type shared")
	}
}
