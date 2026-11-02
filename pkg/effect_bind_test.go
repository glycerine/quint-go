package quint

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBindEffectVariable(t *testing.T) {
	for _, input := range []string{"e", "(e) => Pure", "(Pure) => e", "((Pure) => e) => Pure"} {
		e, err := ParseEffect(input)
		if err != nil {
			t.Fatal(err)
		}
		subs, err := BindEffectVariable("e", e)
		if err == nil || err.Error() != "Can't bind e to "+EffectString(e)+": cyclical binding" || subs != nil {
			t.Fatalf("%s: %v %v", input, subs, err)
		}
	}
	for _, input := range []string{"Pure", "f", "Read[e]", "(Read[e]) => Update[e]"} {
		e, err := ParseEffect(input)
		if err != nil {
			t.Fatal(err)
		}
		subs, err := BindEffectVariable("e", e)
		if err != nil || len(subs) != 1 || subs[0].Name != "e" || subs[0].Kind != "effect" || !reflect.DeepEqual(subs[0].Effect, e) {
			t.Fatalf("%s: %v %v", input, subs, err)
		}
		if e.Result != nil {
			subs[0].Effect.Result.Kind = "variable"
			if e.Result.Kind != "concrete" {
				t.Fatal("aliased result")
			}
		}
	}
}

func TestBindEntityVariable(t *testing.T) {
	v := func(name string) Entity { return Entity{Kind: "variable", Name: name} }
	u := func(children ...Entity) Entity { return Entity{Kind: "union", Entities: children} }
	c := Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x"}}}
	for _, tc := range []struct {
		input Entity
		want  []Entity
	}{
		{v("a"), []Entity{v("a")}},
		{v("b"), []Entity{v("b")}},
		{c, []Entity{c}},
		{u(), []Entity{u()}},
		{u(v("b"), c), []Entity{u(v("b"), c)}},
		{u(v("a"), v("b"), v("a")), []Entity{v("a"), v("b"), v("a")}},
		{u(c, u(v("b"), u(v("a"), v("c"))), u(v("d"), v("e"))), []Entity{c, v("b"), v("a"), v("c"), u(v("d"), v("e"))}},
	} {
		subs := BindEntityVariable("a", tc.input)
		got := []Entity{}
		for _, s := range subs {
			if s.Kind != "entity" || s.Name != "a" {
				t.Fatal(s)
			}
			got = append(got, s.Entity)
		}
		ga, _ := json.Marshal(got)
		wa, _ := json.Marshal(tc.want)
		if string(ga) != string(wa) {
			t.Fatalf("%+v: got %+v want %+v", tc.input, got, tc.want)
		}
	}
	input := u(v("b"), c)
	subs := BindEntityVariable("a", input)
	subs[0].Entity.Entities[1].StateVariables[0].Name = "changed"
	if input.Entities[1].StateVariables[0].Name != "x" {
		t.Fatal("aliased entity")
	}
}
