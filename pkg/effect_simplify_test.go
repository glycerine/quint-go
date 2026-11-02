package quint

import (
	"reflect"
	"testing"
)

func TestFlattenEntityUnions(t *testing.T) {
	a, _ := ParseID("1")
	b, _ := ParseID("2")
	x := Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x", Reference: a}}}
	y := Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x", Reference: b}}}
	variable := Entity{Kind: "variable", Name: "v", Reference: &a}
	cases := []struct{ in, want Entity }{
		{Entity{Kind: "union"}, Entity{Kind: "concrete", StateVariables: []StateVariable{}}},
		{Entity{Kind: "union", Entities: []Entity{variable}}, variable},
		{Entity{Kind: "union", Entities: []Entity{x, {Kind: "union", Entities: []Entity{y}}}}, Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x", Reference: a}, {Name: "x", Reference: b}}}},
		{Entity{Kind: "union", Entities: []Entity{x, variable}}, Entity{Kind: "union", Entities: []Entity{variable, x}}},
	}
	for _, c := range cases {
		got := FlattenEntityUnions(c.in)
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("flatten %+v: %+v want %+v", c.in, got, c.want)
		}
	}
	got := FlattenEntityUnions(x)
	got.StateVariables[0].Name = "changed"
	if x.StateVariables[0].Name != "x" {
		t.Fatal("aliased state variables")
	}
	got = FlattenEntityUnions(variable)
	*got.Reference = b
	if *variable.Reference != a {
		t.Fatal("aliased reference")
	}
}
func TestSimplifyEffectRetainsUpdates(t *testing.T) {
	v := Entity{Kind: "variable", Name: "v"}
	union := Entity{Kind: "union", Entities: []Entity{v, v}}
	concrete := Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "read", Entity: union}, {Kind: "update", Entity: union}, {Kind: "temporal", Entity: union}}}
	e := Effect{Kind: "arrow", Params: []Effect{concrete, {Kind: "variable", Name: "e"}}, Result: &concrete}
	got := SimplifyEffect(e)
	for _, effect := range []Effect{got.Params[0], *got.Result} {
		for i, want := range []int{1, 2, 1} {
			if len(effect.Components[i].Entity.Entities) != want {
				t.Fatalf("component %d lost occurrences", i)
			}
		}
	}
	got.Params[0].Components[0].Entity.Entities[0].Name = "changed"
	if e.Params[0].Components[0].Entity.Entities[0].Name != "v" {
		t.Fatal("mutated input")
	}
	if !reflect.DeepEqual(SimplifyEffect(Effect{Kind: "variable", Name: "e"}), Effect{Kind: "variable", Name: "e"}) {
		t.Fatal("changed effect variable")
	}
}
