package quint

import (
	"strconv"
	"strings"
)

// InstantiateEffectScheme freshens quantified names, retaining free names.
// Composition puts entity bindings first, matching upstream's inference helper.
func InstantiateEffectScheme(s EffectScheme, fresh *FreshNames) (Effect, error) {
	effects, entities := []EffectSubstitution{}, []EffectSubstitution{}
	for _, n := range s.EffectVariables {
		effects = append(effects, EffectSubstitution{Kind: "effect", Name: n, Effect: Effect{Kind: "variable", Name: fresh.Next("_e")}})
	}
	for _, n := range s.EntityVariables {
		entities = append(entities, EffectSubstitution{Kind: "entity", Name: n, Entity: Entity{Kind: "variable", Name: fresh.Next("_v")}})
	}
	subs, err := ComposeEffectSubstitutions(effects, entities)
	if err != nil {
		return Effect{}, err
	}
	return ApplyEffectSubstitution(subs, s.Effect)
}

// EffectSchemeString uses quantifier order to assign canonical diagnostic names.
func EffectSchemeString(s EffectScheme) (string, error) {
	names := []string{}
	subs := []EffectSubstitution{}
	for i, n := range s.EffectVariables {
		name := "e" + strconv.Itoa(i)
		names = append(names, name)
		subs = append(subs, EffectSubstitution{Kind: "effect", Name: n, Effect: Effect{Kind: "variable", Name: name}})
	}
	for i, n := range s.EntityVariables {
		name := "v" + strconv.Itoa(i)
		names = append(names, name)
		subs = append(subs, EffectSubstitution{Kind: "entity", Name: n, Entity: Entity{Kind: "variable", Name: name}})
	}
	e, err := ApplyEffectSubstitution(subs, s.Effect)
	if err != nil {
		return "", err
	}
	prefix := ""
	if len(names) > 0 {
		prefix = "∀ " + strings.Join(names, ", ") + " . "
	}
	return prefix + EffectString(e), nil
}

// AddEffectNamespaces qualifies concrete state variables using the lookup
// table's innermost-first namespace order. Symbolic entity names are unchanged.
func AddEffectNamespaces(effect Effect, namespaces []string) Effect {
	out := cloneEffectSubstitutionValue(effect)
	var entity func(Entity) Entity
	entity = func(e Entity) Entity {
		for i := range e.Entities {
			e.Entities[i] = entity(e.Entities[i])
		}
		if len(namespaces) > 0 {
			prefix := ""
			for _, n := range namespaces {
				prefix = n + "::" + prefix
			}
			for i := range e.StateVariables {
				e.StateVariables[i].Name = prefix + e.StateVariables[i].Name
				// Upstream object spread creates a fresh state-variable occurrence.
				e.StateVariables[i].identity = new(int)
			}
		}
		return e
	}
	var visit func(*Effect)
	visit = func(e *Effect) {
		for i := range e.Params {
			visit(&e.Params[i])
		}
		if e.Result != nil {
			visit(e.Result)
		}
		for i := range e.Components {
			e.Components[i].Entity = entity(e.Components[i].Entity)
		}
	}
	visit(&out)
	return out
}
