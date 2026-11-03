package quint

import (
	"fmt"
	"reflect"
)

// Retain the complete previous preparation and fixed-point formula.
func effectSubstitutionPreparedOracle(subs []EffectSubstitution, input Effect) (Effect, error) {
	subs = append([]EffectSubstitution{}, subs...)
	for i := range subs {
		if subs[i].Kind == "entity" {
			subs[i].Entity = cloneEntity(subs[i].Entity)
		} else if subs[i].Kind == "effect" {
			subs[i].Effect = cloneEffectSubstitutionValue(subs[i].Effect)
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
