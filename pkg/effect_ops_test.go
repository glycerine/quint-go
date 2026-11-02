package quint

import (
	"reflect"
	"testing"
)

func TestEffectPrintingAndNames(t *testing.T) {
	entity := Entity{Kind: "union", Entities: []Entity{{Kind: "concrete", StateVariables: []StateVariable{{Name: "x"}, {Name: "y"}}}, {Kind: "variable", Name: "v"}, {Kind: "union", Entities: []Entity{{Kind: "variable", Name: "v"}, {Kind: "variable", Name: "w"}}}}}
	if got := EntityString(entity); got != "'x', 'y', v, v, w" {
		t.Fatal(got)
	}
	if got := EntityNames(entity); !reflect.DeepEqual(got, []string{"v", "v", "w"}) {
		t.Fatal(got)
	}
	concrete := Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "read", Entity: entity}, {Kind: "update", Entity: Entity{Kind: "variable", Name: "v"}}, {Kind: "temporal", Entity: Entity{Kind: "concrete"}}}}
	e := Effect{Kind: "arrow", Params: []Effect{{Kind: "variable", Name: "e"}, {Kind: "concrete"}, {Kind: "variable", Name: "e"}}, Result: &concrete}
	if got := EffectString(e); got != "(e, Pure, e) => Read['x', 'y', v, v, w] & Update[v] & Temporal[]" {
		t.Fatal(got)
	}
	effects, entities := EffectNames(e)
	if !reflect.DeepEqual(effects, []string{"e"}) || !reflect.DeepEqual(entities, []string{"v", "w"}) {
		t.Fatalf("names: %v %v", effects, entities)
	}
	effects[0] = "changed"
	again, _ := EffectNames(e)
	if again[0] != "e" {
		t.Fatal("mutated shared result")
	}
	if got := EffectString(Effect{Kind: "arrow", Result: &Effect{Kind: "variable", Name: "r"}}); got != "() => r" {
		t.Fatal(got)
	}
	for _, entity := range []Entity{{Kind: "concrete"}, {Kind: "union"}} {
		if EntityString(entity) != "" || len(EntityNames(entity)) != 0 {
			t.Fatal("empty entity handling")
		}
	}
	effects, entities = EffectNames(Effect{Kind: "concrete"})
	if len(effects) != 0 || len(entities) != 0 {
		t.Fatal("pure effect has variables")
	}
}
