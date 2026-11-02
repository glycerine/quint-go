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
		for i := range out.StateVariables {
			if out.StateVariables[i].identity == nil {
				out.StateVariables[i].identity = new(int)
			}
		}
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
// Read/temporal self-unions retain their recursive variable and deduplicate
// shared occurrences. Non-stabilizing cycles return an error.
func ApplyEffectSubstitution(subs []EffectSubstitution, input Effect) (Effect, error) {
	active := map[string]bool{}
	selfUnion := map[string]bool{}
	allowSelfUnion := false
	subs = append([]EffectSubstitution{}, subs...)
	for i := range subs {
		if subs[i].Kind == "entity" {
			subs[i].Entity = cloneEntity(subs[i].Entity)
		}
	}
	var effect func(Effect) (Effect, error)
	var entity func(Entity) (Entity, error)
	entity = func(e Entity) (Entity, error) {
		if e.Kind == "variable" {
			for _, s := range subs {
				if s.Name == e.Name {
					if s.Kind == "entity" && !reflect.DeepEqual(s.Entity, e) {
						key := "entity:" + e.Name
						if active[key] {
							if selfUnion[key] && allowSelfUnion {
								return cloneEntity(e), nil
							}
							return Entity{}, fmt.Errorf("cyclic entity substitution for %s", e.Name)
						}
						active[key] = true
						if s.Entity.Kind == "union" {
							for _, childName := range EntityNames(s.Entity) {
								if childName == e.Name {
									selfUnion[key] = true
								}
							}
						}
						v, err := entity(s.Entity)
						delete(active, key)
						delete(selfUnion, key)
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
				allowSelfUnion = c.Kind != "update"
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
		for {
			simplified := SimplifyEffect(out)
			if reflect.DeepEqual(simplified, out) {
				return simplified, nil
			}
			out = simplified
		}
	}
	return effect(input)
}

// ComposeEffectSubstitutions applies first to second's values and appends first.
// Duplicate names are retained in that order. Entity values receive one pass;
// effect values receive transitive substitution and simplification.
func ComposeEffectSubstitutions(first, second []EffectSubstitution) ([]EffectSubstitution, error) {
	out := make([]EffectSubstitution, 0, len(first)+len(second))
	for _, s := range second {
		switch s.Kind {
		case "effect":
			value, err := ApplyEffectSubstitution(first, s.Effect)
			if err != nil {
				return nil, err
			}
			s.Effect = value
		case "entity":
			s.Entity = ApplyEntitySubstitution(first, s.Entity)
		default:
			return nil, fmt.Errorf("unknown effect substitution kind %q", s.Kind)
		}
		out = append(out, s)
	}
	for _, s := range first {
		switch s.Kind {
		case "effect":
			s.Effect = cloneEffectSubstitutionValue(s.Effect)
		case "entity":
			s.Entity = cloneEntity(s.Entity)
		default:
			return nil, fmt.Errorf("unknown effect substitution kind %q", s.Kind)
		}
		out = append(out, s)
	}
	return out, nil
}

func cloneEffectSubstitutionValue(e Effect) Effect {
	out := e
	if e.Params != nil {
		out.Params = make([]Effect, len(e.Params))
		for i, p := range e.Params {
			out.Params[i] = cloneEffectSubstitutionValue(p)
		}
	}
	if e.Result != nil {
		value := cloneEffectSubstitutionValue(*e.Result)
		out.Result = &value
	}
	if e.Components != nil {
		out.Components = make([]EffectComponent, len(e.Components))
		for i, c := range e.Components {
			out.Components[i] = EffectComponent{Kind: c.Kind, Entity: cloneEntity(c.Entity)}
		}
	}
	return out
}
