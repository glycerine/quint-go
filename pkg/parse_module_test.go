package quint

import "testing"

func TestParseModules(t *testing.T) {
	source := `/// Model docs
module Counter {
 const N: int
 var x: int
 assume positive = N > 0
 type Address
 type Bag[a] = Set[a]
 /// Initialize
 action init = x' = 0
 action step = x' = x + 1
 val invariant = x <= N
 /// orphan doc
}
module Other { pure val ready = true }`
	modules, err := ParseModules(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(modules) != 2 || modules[0].Name != "Counter" || len(modules[0].Declarations) != 8 || modules[0].Doc == nil || *modules[0].Doc != "Model docs" {
		t.Fatalf("modules %+v", modules)
	}
	decls := modules[0].Declarations
	for i, kind := range []string{"const", "var", "assume", "typedef", "typedef", "def", "def", "def"} {
		if decls[i].Kind != kind {
			t.Fatalf("declaration %d: %+v", i, decls[i])
		}
	}
	if decls[4].Params[0] != "a" || decls[4].Type.Kind != "set" || decls[5].Doc == nil || *decls[5].Doc != "Initialize" || decls[6].Doc != nil {
		t.Fatal("type parameters or doc attachment")
	}
	seen := map[ID]bool{}
	for _, m := range modules {
		if seen[m.ID] {
			t.Fatal("duplicate module ID")
		}
		seen[m.ID] = true
		for _, d := range m.Declarations {
			if seen[d.ID] {
				t.Fatal("duplicate declaration ID")
			}
			seen[d.ID] = true
		}
	}
}
func TestModuleErrors(t *testing.T) {
	for _, s := range []string{"", "module M { var x }", "module M { val x = }", "module M {} trailing"} {
		if _, err := ParseModules(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
	modules, err := ParseModules("module Empty {}")
	if err != nil || len(modules) != 1 || len(modules[0].Declarations) != 0 {
		t.Fatalf("empty module: %v %v", modules, err)
	}
}

func TestModuleImportsAndExports(t *testing.T) {
	for _, tc := range []struct{ source, kind, proto, member, alias, path string }{
		{`import A`, "import", "A", "", "", ""},
		{`import A.x from "./a"`, "import", "A", "x", "", "./a"},
		{`import A.*`, "import", "A", "*", "", ""},
		{`import A::B as C from "../b"`, "import", "A::B", "", "C", "../b"},
		{`export A`, "export", "A", "", "", ""},
		{`export A.x`, "export", "A", "x", "", ""},
		{`export A.*`, "export", "A", "*", "", ""},
		{`export A as B`, "export", "A", "", "B", ""},
	} {
		t.Run(tc.source, func(t *testing.T) {
			ms, err := ParseModules("module M { " + tc.source + " }")
			if err != nil {
				t.Fatal(err)
			}
			d := ms[0].Declarations[0]
			check := func(label string, got *string, want string) {
				t.Helper()
				if want == "" {
					if got != nil {
						t.Errorf("unexpected %s: %q", label, *got)
					}
					return
				}
				if got == nil || *got != want {
					t.Errorf("%s: %v, want %q", label, got, want)
				}
			}
			if d.Kind != tc.kind || d.ProtoName != tc.proto {
				t.Fatalf("declaration %+v", d)
			}
			check("member", d.DefName, tc.member)
			check("alias", d.QualifiedName, tc.alias)
			check("source", d.FromSource, tc.path)
		})
	}
}

func TestModuleInstances(t *testing.T) {
	for _, suffix := range []string{`.*`, `as Local`} {
		ms, err := ParseModules(`module M { import A::B(N = 3, flag = true,) ` + suffix + ` from "./spec" }`)
		if err != nil {
			t.Fatal(err)
		}
		d := ms[0].Declarations[0]
		if d.Kind != "instance" || d.ProtoName != "A::B" || d.FromSource == nil || *d.FromSource != "./spec" || len(d.Overrides) != 2 {
			t.Fatalf("instance %+v", d)
		}
		if d.IdentityOverride != (suffix == ".*") {
			t.Fatal("identity override")
		}
		if suffix == ".*" {
			if d.QualifiedName != nil {
				t.Fatal("wildcard alias")
			}
		} else if d.QualifiedName == nil || *d.QualifiedName != "Local" {
			t.Fatal("instance alias")
		}
		if d.Overrides[0].Parameter.Name != "N" || d.Overrides[0].Expr.Kind != "int" || d.Overrides[1].Parameter.Name != "flag" || d.Overrides[1].Expr.Kind != "bool" {
			t.Fatal("override order or expressions")
		}
		seen := map[ID]bool{ms[0].ID: true, d.ID: true}
		for _, o := range d.Overrides {
			for _, id := range []ID{o.Parameter.ID, o.Expr.ID} {
				if seen[id] {
					t.Fatal("duplicate ID")
				}
				seen[id] = true
			}
		}
	}
	for _, source := range []string{`import A() as B`, `import A(N =) as B`, `import A(N = 1)`, `export A from "a"`} {
		if _, err := ParseModules("module M { " + source + " }"); err == nil {
			t.Errorf("accepted malformed declaration %q", source)
		}
	}
}
