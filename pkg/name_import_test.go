package quint

import "testing"

func TestImportDefinitionForms(t *testing.T) {
	ms, err := ParseModules(`module A { const N: int val x = 1 type T = int }`)
	if err != nil {
		t.Fatal(err)
	}
	source := CollectLocalNames(ms).Modules.Get("A")
	for _, tc := range []struct {
		text, key string
		count     int
	}{
		{"import A", "A::x", 3}, {"import A as B", "B::x", 3}, {"import A.*", "x", 3}, {"import A.x", "x", 1},
	} {
		ds, err := ParseDeclaration(tc.text)
		if err != nil {
			t.Fatal(err)
		}
		dest := NewNameScope("M")
		dest.ImportDefinitions(source, ds[0])
		def, ok := dest.Lookup(tc.key)
		if !ok || !def.Hidden || def.ImportedFrom == nil || def.ID != ms[0].Declarations[1].ID || len(dest.Errors) != 0 {
			t.Fatalf("%s: %+v %v", tc.text, def, dest.Errors)
		}
		count := 0
		for range dest.Definitions.All() {
			count++
		}
		if count != tc.count {
			t.Fatal("import count")
		}
		original, _ := source.Lookup("x")
		if original.Hidden || original.ImportedFrom != nil || len(original.Namespaces) != 0 {
			t.Fatal("source mutated")
		}
	}
}
func TestImportsAreNotTransitiveExceptConstants(t *testing.T) {
	ms, _ := ParseModules(`module A { const N: int val x = 1 }`)
	source := CollectLocalNames(ms).Modules.Get("A")
	ds, _ := ParseDeclaration("import A.*")
	middle := NewNameScope("B")
	middle.ImportDefinitions(source, ds[0])
	ds, _ = ParseDeclaration("import B.*")
	dest := NewNameScope("C")
	dest.ImportDefinitions(middle, ds[0])
	if _, ok := dest.Lookup("x"); ok {
		t.Fatal("hidden definition reimported")
	}
	if _, ok := dest.Lookup("N"); !ok {
		t.Fatal("constant not propagated")
	}
}
func TestImportDefinitionErrors(t *testing.T) {
	for _, tc := range []struct{ text, code string }{{"import M", QNT407}, {"import Missing", QNT405}, {"import A.missing", QNT404}} {
		ds, _ := ParseDeclaration(tc.text)
		dest := NewNameScope("M")
		var source *NameScope
		if tc.code == QNT404 {
			source = NewNameScope("A")
		}
		dest.ImportDefinitions(source, ds[0])
		if len(dest.Errors) != 1 || dest.Errors[0].Code != tc.code {
			t.Fatalf("%s: %v", tc.text, dest.Errors)
		}
	}
}
