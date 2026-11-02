package quint

import (
	"reflect"
	"testing"
)

func TestDeclarationEntryPoint(t *testing.T) {
	for _, tc := range []struct {
		source, kind string
		count        int
	}{
		{`const N: int`, "const", 1}, {`var x: int`, "var", 1},
		{`assume _ = true`, "assume", 1}, {`pure def add(x: int): int = x + 1`, "def", 1},
		{`type T`, "typedef", 1}, {`type Bag[a] = List[a]`, "typedef", 1},
		{`type Option[a] = Some(a) | None`, "typedef", 3},
		{`val (x,_,z) = triple`, "def", 3}, {`pure val {x} = record`, "def", 2},
		{`import A.* from "./a"`, "import", 1}, {`export A as B`, "export", 1},
		{`import A(N=3) as B`, "instance", 1},
	} {
		t.Run(tc.source, func(t *testing.T) {
			ds, err := ParseDeclaration(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			if len(ds) != tc.count || ds[0].Kind != tc.kind {
				t.Fatalf("declarations %+v", ds)
			}
			ms, err := ParseModules("module M { " + tc.source + " }")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(ds, ms[0].Declarations) {
				t.Fatal("entry point differs from module conversion")
			}
		})
	}
}

func TestDeclarationDocumentationAndErrors(t *testing.T) {
	ds, err := ParseDeclaration("// regular comment\n/// first\n/// second\ntype T = A | B\n// trailing comment\n")
	if err != nil {
		t.Fatal(err)
	}
	if ds[0].Doc == nil || *ds[0].Doc != "first\nsecond" || ds[1].Doc != nil || ds[2].Doc != nil {
		t.Fatal("documentation attachment")
	}
	for _, source := range []string{"", "/// docs only", "val x =", "val x = 1 val y = 2", "module M {}", "var x: int junk", "val x = 1\n/// trailing"} {
		if ds, err := ParseDeclaration(source); err == nil || ds != nil {
			t.Errorf("accepted %q: %v %v", source, ds, err)
		}
	}
}
