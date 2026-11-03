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

// ApplyEffectSubstitution applies one pass, simplifies, and repeats until the
// whole effect is unchanged. Simplification must happen between passes: resolving
// a self-containing union recursively changes both its members and its shape.
func ApplyEffectSubstitution(subs []EffectSubstitution, input Effect) (Effect, error) {
	if effectSubstitutionsNeedPreparation(subs) {
		subs = append([]EffectSubstitution{}, subs...)
		for i := range subs {
			if subs[i].Kind == "entity" {
				subs[i].Entity = cloneEntity(subs[i].Entity)
			} else if subs[i].Kind == "effect" {
				subs[i].Effect = cloneEffectSubstitutionValue(subs[i].Effect)
			}
		}
	}
	var apply func(Effect) (Effect, error)
	apply = func(input Effect) (Effect, error) {
		current := cloneEffectSubstitutionValue(input)
		history := []Effect{}
		growthChecked := false
		for {
			next := current
			switch current.Kind {
			case "variable":
				for _, s := range subs {
					if s.Name == current.Name {
						if s.Kind == "effect" {
							next = cloneEffectSubstitutionValue(s.Effect)
						}
						break
					}
				}
			case "concrete":
				next.Components = []EffectComponent{}
				for _, c := range current.Components {
					entity := ApplyEntitySubstitution(subs, c.Entity)
					// Upstream removes empty entities before simplification.
					if entity.Kind == "concrete" && len(entity.StateVariables) == 0 ||
						entity.Kind == "union" && len(entity.Entities) == 0 {
						continue
					}
					next.Components = append(next.Components, EffectComponent{Kind: c.Kind, Entity: entity})
				}
			case "arrow":
				next.Params = make([]Effect, len(current.Params))
				for i, p := range current.Params {
					v, err := apply(p)
					if err != nil {
						return Effect{}, err
					}
					next.Params[i] = v
				}
				v, err := apply(*current.Result)
				if err != nil {
					return Effect{}, err
				}
				next.Result = &v
			}
			next = SimplifyEffect(next)
			if reflect.DeepEqual(next, current) {
				return next, nil
			}
			for _, previous := range history {
				if reflect.DeepEqual(next, previous) {
					return Effect{}, fmt.Errorf("cyclic effect substitution")
				}
			}
			history = append(history, current)
			current = next
			// Ordinary substitutions stabilize in a few passes. Only inspect
			// dependency cycles when repeated passes may expand forever.
			if len(history) >= 3 && !growthChecked && current.Kind != "variable" {
				if err := checkEffectSubstitutionGrowth(subs, current); err != nil {
					return Effect{}, err
				}
				growthChecked = true
			}
		}
	}
	// Check nested effect-variable cycles before recursively visiting arrows.
	if err := checkEffectArrowCycles(subs, input); err != nil {
		return Effect{}, err
	}
	return apply(input)
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
