package quint

import (
	"reflect"
	"testing"
)

func TestCollectLocalDefinitions(t *testing.T) {
	modules, err := ParseModules(`module M {
 const N: int var x: int type T = int assume positive = N > 0 assume _ = true
 def f(y: int): int = val local = y; local
 import Other export Other import Other(N=1) as Instance
 }`)
	if err != nil {
		t.Fatal(err)
	}
	collection := CollectLocalNames(modules)
	if len(collection.Errors) != 0 {
		t.Fatal(collection.Errors)
	}
	scope := collection.Modules.Get("M")
	names := []string{}
	for name := range scope.Definitions.All() {
		names = append(names, name)
	}
	if !reflect.DeepEqual(names, []string{"N", "x", "T", "positive", "f"}) {
		t.Fatal(names)
	}
	if _, ok := scope.Lookup("local"); ok {
		t.Fatal("local definition leaked")
	}
	f, _ := scope.Lookup("f")
	if f.Depth != 0 || f.Declaration.Depth == nil || f.Declaration.TypeAnnotation != nil {
		t.Fatal("operator collection metadata")
	}
	if modules[0].Declarations[5].TypeAnnotation == nil || modules[0].Declarations[5].Depth != nil {
		t.Fatal("input IR mutated")
	}
	n, _ := scope.Lookup("N")
	if n.Declaration.TypeAnnotation == nil {
		t.Fatal("constant annotation dropped")
	}
	if len(collection.Pending.Get("M")) != 3 {
		t.Fatal("module directives lost")
	}
}
func TestCollectLocalConflictsAndModules(t *testing.T) {
	modules, err := ParseModules(`module M { val x = 1 val x = 2 val map = 3 } module N { val x = 4 } module M { val y = 5 }`)
	if err != nil {
		t.Fatal(err)
	}
	c := CollectLocalNames(modules)
	if len(c.Errors) != 4 || c.Errors[0].Code != QNT101 || c.Errors[2].Code != QNT101 || c.Errors[3].Code != QNT102 {
		t.Fatalf("diagnostics %+v", c.Errors)
	}
	if c.Errors[3].Message != "Module with name 'M' was already defined" || *c.Errors[3].Reference != modules[2].ID {
		t.Fatal("duplicate module diagnostic")
	}
	if _, ok := c.Modules.Get("M").Lookup("y"); !ok {
		t.Fatal("last module scope should replace previous")
	}
	names := []string{}
	for name := range c.Modules.All() {
		names = append(names, name)
	}
	if !reflect.DeepEqual(names, []string{"M", "N"}) {
		t.Fatal("module insertion order", names)
	}
	if c.Modules.Get("N").Errors == nil || len(c.Modules.Get("N").Errors) != 0 {
		t.Fatal("module scopes leaked")
	}
}
