package quint

import (
	"reflect"
	"testing"
)

func TestSortModuleLayers(t *testing.T) {
	modules, err := ParseModules(`module Main { import A import B } module A { import C } module B { import C } module C {}`)
	if err != nil {
		t.Fatal(err)
	}
	original := append([]Module(nil), modules...)
	sorted, ds := SortModules(modules)
	if len(ds) != 0 {
		t.Fatal(ds)
	}
	names := []string{}
	for _, m := range sorted {
		names = append(names, m.Name)
	}
	if !reflect.DeepEqual(names, []string{"C", "A", "B", "Main"}) {
		t.Fatal(names)
	}
	if !reflect.DeepEqual(original, modules) {
		t.Fatal("mutated input")
	}
	// JavaScript's default sort is lexical even for BigInt values.
	for i, s := range []string{"2", "10", "3"} {
		id, _ := ParseID(s)
		modules[i].ID = id
		modules[i].Declarations = nil
	}
	sorted, ds = SortModules(modules[:3])
	if len(ds) != 0 || sorted[0].ID.String() != "10" || sorted[1].ID.String() != "2" {
		t.Fatal("ID ordering")
	}
}
func TestSortModuleDiagnostics(t *testing.T) {
	for _, tc := range []struct{ source, code, message string }{
		{`module A {} module A {}`, QNT101, "Multiple modules conflict on the same name: A"},
		{`module A { import Missing }`, QNT405, "Module A imports an unknown module Missing"},
		{`module A { import A }`, QNT098, "Cyclic imports among: A"},
		{`module A { import B } module B { import A } module C { import A }`, QNT098, "Cyclic imports among: A, B, C"},
	} {
		ms, err := ParseModules(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		sorted, ds := SortModules(ms)
		if len(ds) != 1 || ds[0].Code != tc.code || ds[0].Message != tc.message || ds[0].Reference == nil || !reflect.DeepEqual(sorted, ms) {
			t.Fatalf("%s: %+v", tc.source, ds)
		}
	}
	ms, err := ParseModules(`module A { export Missing }`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ds := SortModules(ms); len(ds) != 0 {
		t.Fatal("export must not create import edge")
	}
	if ms, ds := SortModules(nil); len(ms) != 0 || len(ds) != 0 {
		t.Fatal("empty graph")
	}
}
