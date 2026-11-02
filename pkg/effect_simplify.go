package quint

import "reflect"

// FlattenEntityUnions follows upstream's recursive grouping order. Distinct
// state-variable occurrences retain their references, including repeated names.
func FlattenEntityUnions(e Entity) Entity {
	if e.Kind != "union" {
		return cloneEntity(e)
	}
	entities := []Entity{}
	vars := []StateVariable{}
	for _, child := range e.Entities {
		flat := FlattenEntityUnions(child)
		switch flat.Kind {
		case "variable":
			entities = append(entities, flat)
		case "concrete":
			vars = append(vars, flat.StateVariables...)
		case "union":
			entities = append(entities, flat.Entities...)
		}
	}
	if len(entities) == 0 {
		return Entity{Kind: "concrete", StateVariables: vars}
	}
	if len(vars) > 0 {
		entities = append(entities, Entity{Kind: "concrete", StateVariables: vars})
	}
	if len(entities) == 1 {
		return entities[0]
	}
	return Entity{Kind: "union", Entities: entities}
}
func deduplicateEffectEntity(e Entity) Entity {
	if e.Kind == "concrete" {
		out := cloneEntity(e)
		out.StateVariables = []StateVariable{}
		seen := map[*int]bool{}
		for _, v := range e.StateVariables {
			if v.identity == nil || !seen[v.identity] {
				out.StateVariables = append(out.StateVariables, v)
				seen[v.identity] = true
			}
		}
		return out
	}
	if e.Kind != "union" {
		return e
	}
	out := e
	out.Entities = []Entity{}
	for _, child := range e.Entities {
		child = deduplicateEffectEntity(child)
		exists := false
		for _, old := range out.Entities {
			if reflect.DeepEqual(old, child) {
				exists = true
				break
			}
		}
		if !exists {
			out.Entities = append(out.Entities, child)
		}
	}
	return out
}

// SimplifyEffect removes structurally repeated union members for reads and
// temporal effects. Updates retain every occurrence for multiple-update checks.
// Internal occurrence tokens preserve JS object-identity Set behavior: copied
// occurrences deduplicate, while distinct equal-valued occurrences remain.
func SimplifyEffect(e Effect) Effect {
	out := e
	switch e.Kind {
	case "concrete":
		out.Components = make([]EffectComponent, len(e.Components))
		for i, c := range e.Components {
			entity := FlattenEntityUnions(c.Entity)
			if c.Kind != "update" {
				entity = deduplicateEffectEntity(entity)
			}
			out.Components[i] = EffectComponent{Kind: c.Kind, Entity: entity}
		}
	case "arrow":
		out.Params = make([]Effect, len(e.Params))
		for i, p := range e.Params {
			out.Params[i] = SimplifyEffect(p)
		}
		if e.Result != nil {
			r := SimplifyEffect(*e.Result)
			out.Result = &r
		}
	}
	return out
}
