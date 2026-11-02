package quint

import "testing"

func TestExportVisibilityAndReplacement(t *testing.T) {
	ms, err := ParseModules(`module A { val x = 1 const N: int }`)
	if err != nil {
		t.Fatal(err)
	}
	source := CollectLocalNames(ms).Modules.Get("A")
	for _, tc := range []struct{ directive, key string }{{"export A.*", "x"}, {"export A.x", "x"}, {"export A", "A::x"}, {"export A as Public", "Public::x"}} {
		ds, err := ParseDeclaration(tc.directive)
		if err != nil {
			t.Fatal(err)
		}
		middle := NewNameScope("B")
		middle.ExportDefinitions(source, ds[0])
		def, ok := middle.Lookup(tc.key)
		if !ok || def.Hidden || def.ImportedFrom.Kind != "export" {
			t.Fatalf("export %+v", def)
		}
		imports, _ := ParseDeclaration("import B.*")
		dest := NewNameScope("C")
		dest.ImportDefinitions(middle, imports[0])
		if got, ok := dest.Lookup(tc.key); !ok || got.ID != ms[0].Declarations[0].ID {
			t.Fatal("re-export identity")
		}
	}
	middle := NewNameScope("B")
	imp, _ := ParseDeclaration("import A.*")
	middle.ImportDefinitions(source, imp[0])
	exp, _ := ParseDeclaration("export A.*")
	middle.ExportDefinitions(source, exp[0])
	if len(middle.Errors) != 0 || len(middle.Definitions.Get("x")) != 1 {
		t.Fatal("same-ID export conflicts")
	}
	if def, _ := middle.Lookup("x"); def.Hidden {
		t.Fatal("export did not replace hidden import")
	}
}
func TestExportErrorsAndConflicts(t *testing.T) {
	ds, _ := ParseDeclaration("export M.*")
	dest := NewNameScope("M")
	dest.ExportDefinitions(nil, ds[0])
	if len(dest.Errors) != 1 || dest.Errors[0].Code != QNT407 || dest.Errors[0].Message != "Cannot export 'M' inside 'M'" {
		t.Fatal(dest.Errors)
	}
	ms, _ := ParseModules(`module A { val x = 1 } module B { val x = 2 }`)
	scopes := CollectLocalNames(ms)
	dest = scopes.Modules.Get("B")
	exp, _ := ParseDeclaration("export A.x")
	dest.ExportDefinitions(scopes.Modules.Get("A"), exp[0])
	if len(dest.Errors) != 2 || *dest.Errors[0].Reference != ms[1].Declarations[0].ID || *dest.Errors[1].Reference != exp[0].ID {
		t.Fatalf("selective conflict %+v", dest.Errors)
	}
	empty := NewNameScope("C")
	exp, _ = ParseDeclaration("export A.absent")
	empty.ExportDefinitions(scopes.Modules.Get("A"), exp[0])
	if len(empty.Errors) != 1 || empty.Errors[0].Code != QNT404 {
		t.Fatal("missing member")
	}
}
