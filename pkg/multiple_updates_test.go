package quint

import (
	"reflect"
	"testing"
)

func TestMultipleUpdates(t *testing.T) {
	a, _ := ParseID("1")
	b, _ := ParseID("2")
	d, _ := ParseID("3")
	update := func(vars ...StateVariable) Effect {
		return Effect{Kind: "concrete", Components: []EffectComponent{{Kind: "update", Entity: Entity{Kind: "union", Entities: []Entity{{Kind: "concrete", StateVariables: vars}, {Kind: "variable", Name: "unknown"}}}}}}
	}
	safe := update(StateVariable{Name: "x", Reference: a})
	var checker MultipleUpdatesChecker
	result := checker.Check([]EffectScheme{{Effect: Effect{Kind: "arrow", Params: []Effect{safe}, Result: &safe}}})
	n := 0
	for range result.All() {
		n++
	}
	if n != 0 {
		t.Fatal("combined distinct arrow effects")
	}
	bad := update(StateVariable{Name: "x", Reference: a}, StateVariable{Name: "y", Reference: d}, StateVariable{Name: "x", Reference: b})
	checker.Check([]EffectScheme{{Effect: bad}})
	var ids []ID
	for id, diag := range checker.Errors.All() {
		ids = append(ids, id)
		if diag.Code != QNT202 || diag.Message != "Multiple updates of variable x" || diag.Reference == nil || *diag.Reference != id {
			t.Fatalf("diagnostic %+v", diag)
		}
	}
	if !reflect.DeepEqual(ids, []ID{a, b}) {
		t.Fatal(ids)
	}
	bad.Components[0].Kind = "read"
	checker.Check([]EffectScheme{{Effect: bad}})
	ids = nil
	for id := range checker.Errors.All() {
		ids = append(ids, id)
	}
	if !reflect.DeepEqual(ids, []ID{a, b}) {
		t.Fatal("lost accumulated diagnostics")
	}
	vars := EntityStateVariables(Entity{Kind: "concrete", StateVariables: []StateVariable{{Name: "x"}}})
	vars[0].Name = "changed"
	if len(EntityStateVariables(Entity{Kind: "variable", Name: "v"})) != 0 {
		t.Fatal("symbolic entity became concrete")
	}
}
