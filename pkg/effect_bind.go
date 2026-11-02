package quint

import "fmt"

// BindEffectVariable checks effect-variable occurrences before binding. Entity
// variables occupy a separate namespace. The unifier handles equality first.
func BindEffectVariable(name string, effect Effect) ([]EffectSubstitution, error) {
	names, _ := EffectNames(effect)
	for _, n := range names {
		if n == name {
			return nil, fmt.Errorf("Can't bind %s to %s: cyclical binding", name, EffectString(effect))
		}
	}
	return []EffectSubstitution{{Kind: "effect", Name: name, Effect: cloneEffectSubstitutionValue(effect)}}, nil
}

// BindEntityVariable preserves upstream's set-unification rule: a union that
// contains the bound name produces a binding for each member, recursively.
// Self bindings and repeated names are retained for the unifier to consume.
func BindEntityVariable(name string, entity Entity) []EffectSubstitution {
	if entity.Kind == "union" {
		for _, n := range EntityNames(entity) {
			if n == name {
				out := []EffectSubstitution{}
				for _, child := range entity.Entities {
					out = append(out, BindEntityVariable(name, child)...)
				}
				return out
			}
		}
	}
	return []EffectSubstitution{{Kind: "entity", Name: name, Entity: cloneEntity(entity)}}
}
