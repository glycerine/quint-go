package quint

import "testing"

func TestRemoveRunDefinitions(t *testing.T) {
	ms, err := ParseModules("module M { const c:int val a=1 run test=true action step=true }")
	if err != nil {
		t.Fatal(err)
	}
	input := ms[0]
	got := RemoveRunDefinitions(input)
	if len(got.Declarations) != 3 || got.Declarations[0].Name != "c" || got.Declarations[1].Name != "a" || got.Declarations[2].Name != "step" {
		t.Fatal(got)
	}
	if len(input.Declarations) != 4 {
		t.Fatal("input changed")
	}
	got.Declarations[0].Name = "changed"
	if input.Declarations[0].Name != "c" {
		t.Fatal("shared declaration slice")
	}
	if twice := RemoveRunDefinitions(got); len(twice.Declarations) != 3 {
		t.Fatal("not idempotent")
	}
	empty := RemoveRunDefinitions(Module{})
	if len(empty.Declarations) != 0 {
		t.Fatal(empty)
	}
}
