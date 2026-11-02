package quint

import (
	"reflect"
	"testing"
)

func TestSchemeInstantiation(t *testing.T) {
	typ, err := ParseType("(a,{x:b|r})=>a")
	if err != nil {
		t.Fatal(err)
	}
	scheme := TypeScheme{Type: typ, TypeVariables: []string{"a"}, RowVariables: []string{"r"}}
	ts, rs, err := FreeSchemeVariables(scheme)
	if err != nil || !reflect.DeepEqual(ts, []string{"b"}) || len(rs) != 0 {
		t.Fatalf("%v %v %v", ts, rs, err)
	}
	fresh := &FreshNames{}
	u := TypeUnifier{}
	first, e := u.InstantiateScheme(scheme, fresh)
	if e != nil {
		t.Fatal(e)
	}
	second, e := u.InstantiateScheme(scheme, fresh)
	if e != nil {
		t.Fatal(e)
	}
	a, r, _ := TypeNames(&first)
	if !reflect.DeepEqual(a, []string{"_t0", "b"}) || !reflect.DeepEqual(r, []string{"_t1"}) {
		t.Fatalf("%v %v", a, r)
	}
	a, r, _ = TypeNames(&second)
	if !reflect.DeepEqual(a, []string{"_t2", "b"}) || !reflect.DeepEqual(r, []string{"_t3"}) {
		t.Fatalf("%v %v", a, r)
	}
	first.Args[0].Name = "changed"
	if scheme.Type.Args[0].Name != "a" || second.Args[0].Name != "_t2" {
		t.Fatal("scheme mutated")
	}
}
func TestSchemeNamespacesAndAnnotationGenerality(t *testing.T) {
	typ, _ := ParseType("{x:a|r}")
	subs := []Substitution{
		{Kind: "row", Name: "r", Row: Row{Kind: "empty"}},
		{Kind: "type", Name: "a", Type: Type{Kind: "int"}},
		{Kind: "type", Name: "unused", Type: Type{Kind: "str"}},
	}
	e := CheckAnnotationGenerality(subs, typ)
	if e == nil || len(e.Children) != 2 || e.Children[0].Location != "Checking variable a" || e.Children[1].Location != "Checking variable r" {
		t.Fatalf("%+v", e)
	}
	if *e.Children[0].Message != "Type annotation is too general: a should be int" {
		t.Fatal(e)
	}
	if CheckAnnotationGenerality([]Substitution{{Kind: "type", Name: "a", Type: Type{Kind: "var", Name: "b"}}}, typ) != nil {
		t.Fatal("variable renaming rejected")
	}
	same, _ := ParseType("{x:a|a}")
	ts, rs, err := FreeSchemeVariables(TypeScheme{Type: same, TypeVariables: []string{"a"}})
	if err != nil || len(ts) != 0 || !reflect.DeepEqual(rs, []string{"a"}) {
		t.Fatal("namespaces merged")
	}
	_, failure := (TypeUnifier{}).InstantiateScheme(TypeScheme{Type: same, TypeVariables: []string{"a"}, RowVariables: []string{"a"}}, &FreshNames{})
	if failure == nil {
		t.Fatal("upstream incompatible-kind composition not detected")
	}
}
