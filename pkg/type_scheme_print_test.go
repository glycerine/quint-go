package quint

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestCanonicalTypeScheme(t *testing.T) {
	typ, err := ParseType("(a,{x:b|row})=>a")
	if err != nil {
		t.Fatal(err)
	}
	scheme := TypeScheme{Type: typ, TypeVariables: []string{"b", "a"}, RowVariables: []string{"row"}}
	before, _ := json.Marshal(scheme)
	names, out, e := CanonicalTypeScheme(scheme)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(names, []string{"t0", "t1", "r0"}) {
		t.Fatal(names)
	}
	if out.Args[0].Name != "t1" || out.Args[1].Fields.Fields[0].Type.Name != "t0" || out.Args[1].Fields.Other.Name != "r0" {
		t.Fatal(out)
	}
	text, e := TypeSchemeString(scheme)
	if e != nil || text != "∀ t0, t1, r0 . (t1, { x: t0 | r0 }) => t1" {
		t.Fatal(text, e)
	}
	after, _ := json.Marshal(scheme)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
}
func TestSchemePrintingFreeAndEmptyQuantifiers(t *testing.T) {
	typ, _ := ParseType("List[free]")
	text, e := TypeSchemeString(TypeScheme{Type: typ})
	if e != nil || text != "List[free]" {
		t.Fatal(text, e)
	}
	typ, _ = ParseType("{x:free|tail}")
	text, e = TypeSchemeString(TypeScheme{Type: typ, RowVariables: []string{"tail"}})
	if e != nil || text != "∀ r0 . { x: free | r0 }" {
		t.Fatal(text, e)
	}
	_, e = TypeSchemeString(TypeScheme{Type: typ, TypeVariables: []string{"tail"}, RowVariables: []string{"tail"}})
	if e == nil {
		t.Fatal("cross-kind collision")
	}
}
