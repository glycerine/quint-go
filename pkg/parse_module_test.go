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
