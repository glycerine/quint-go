package quint

import (
	"github.com/glycerine/insdict"
	"testing"
)

func TestExpandModuleExports(t *testing.T) {
	ms, err := ParseModules("module A { val a=1 val unused=2 } module B { import A.* export A.* } module C { import B.* val c=a }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	modules := insdict.NewDict[string, Module]()
	for _, m := range ms {
		modules.Set(m.Name, m)
	}
	got, err := ExpandModuleExports(ms[1], modules, linked.Table)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, d := range got.Declarations {
		if d.Name == "a" {
			found = true
			if d.Hidden == nil || *d.Hidden || len(d.ImportedFrom) != 0 {
				t.Fatal(d)
			}
		}
		if d.Kind == "export" {
			t.Fatal("export retained")
		}
	}
	if !found {
		t.Fatal(got)
	}
	if ms[1].Declarations[1].Kind != "export" {
		t.Fatal("source mutated")
	}
}
