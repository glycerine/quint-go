package quint

import (
	"fmt"
	"reflect"
)

type EffectSubstitution struct {
	Kind, Name string
	Effect     Effect
	Entity     Entity
}

// ApplyEntitySubstitution performs one substitution pass, as upstream does.
// Replacement values are copied; transitive closure is the effect-level job.
func ApplyEntitySubstitution(subs []EffectSubstitution, e Entity) Entity {
	if e.Kind == "variable" {
		for _, s := range subs {
			if s.Name == e.Name {
				if s.Kind == "entity" {
					return cloneEntity(s.Entity)
				}
				break
			}
		}
	}
	out := cloneEntity(e)
	for i := range out.Entities {
		out.Entities[i] = ApplyEntitySubstitution(subs, out.Entities[i])
	}
	return out
}
func cloneEntity(e Entity) Entity {
	out := e
	if e.Reference != nil {
		id := *e.Reference
		out.Reference = &id
	}
	if e.StateVariables != nil {
		out.StateVariables = append([]StateVariable{}, e.StateVariables...)
	}
	if e.Entities != nil {
		out.Entities = make([]Entity, len(e.Entities))
		for i, v := range e.Entities {
			out.Entities[i] = cloneEntity(v)
		}
	}
	return out
}

// ApplyEffectSubstitution resolves effect/entity chains and simplifies effects.
// Cyclic replacements return an error instead of diverging.
func ApplyEffectSubstitution(subs []EffectSubstitution, input Effect) (Effect, error) {
	active := map[string]bool{}
	var effect func(Effect) (Effect, error)
	var entity func(Entity) (Entity, error)
	entity = func(e Entity) (Entity, error) {
		if e.Kind == "variable" {
			for _, s := range subs {
				if s.Name == e.Name {
					if s.Kind == "entity" && !reflect.DeepEqual(s.Entity, e) {
						key := "entity:" + e.Name
						if active[key] {
							return Entity{}, fmt.Errorf("cyclic entity substitution for %s", e.Name)
						}
						active[key] = true
						v, err := entity(s.Entity)
						delete(active, key)
						return v, err
					}
					break
				}
			}
		}
		out := cloneEntity(e)
		for i, v := range out.Entities {
			x, err := entity(v)
			if err != nil {
				return Entity{}, err
			}
			out.Entities[i] = x
		}
		return out, nil
	}
	effect = func(e Effect) (Effect, error) {
		if e.Kind == "variable" {
			for _, s := range subs {
				if s.Name == e.Name {
					if s.Kind == "effect" && !reflect.DeepEqual(s.Effect, e) {
						key := "effect:" + e.Name
						if active[key] {
							return Effect{}, fmt.Errorf("cyclic effect substitution for %s", e.Name)
						}
						active[key] = true
						v, err := effect(s.Effect)
						delete(active, key)
						return v, err
					}
					break
				}
			}
		}
		out := e
		if e.Kind == "arrow" {
			out.Params = make([]Effect, len(e.Params))
			for i, p := range e.Params {
				v, err := effect(p)
				if err != nil {
					return Effect{}, err
				}
				out.Params[i] = v
			}
			v, err := effect(*e.Result)
			if err != nil {
				return Effect{}, err
			}
			out.Result = &v
		}
		if e.Kind == "concrete" {
			out.Components = []EffectComponent{}
			for _, c := range e.Components {
				v, err := entity(c.Entity)
				if err != nil {
					return Effect{}, err
				}
				v = FlattenEntityUnions(v)
				if v.Kind == "concrete" && len(v.StateVariables) == 0 {
					continue
				}
				out.Components = append(out.Components, EffectComponent{Kind: c.Kind, Entity: v})
			}
		}
		return SimplifyEffect(out), nil
	}
	return effect(input)
}
