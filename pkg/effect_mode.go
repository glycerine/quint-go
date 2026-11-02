package quint

import "strings"

// ConcreteEffectMode classifies a non-arrow effect for qualifier validation.
// Quantified entity names count as state interactions; free names do not.
func ConcreteEffectMode(s EffectScheme) (Qualifier, string) {
	for _, kind := range []string{"temporal", "update", "read"} {
		relevant := false
		entities := []string{}
		for _, c := range s.Effect.Components {
			if c.Kind != kind {
				continue
			}
			entities = append(entities, EntityString(c.Entity))
			for _, n := range EntityNames(c.Entity) {
				for _, q := range s.EntityVariables {
					if n == q {
						relevant = true
					}
				}
			}
			var concrete func(Entity) bool
			concrete = func(e Entity) bool {
				if len(e.StateVariables) > 0 {
					return true
				}
				for _, child := range e.Entities {
					if concrete(child) {
						return true
					}
				}
				return false
			}
			relevant = relevant || concrete(c.Entity)
		}
		if relevant {
			mode, description := Val, "reads"
			if kind == "update" {
				mode, description = Action, "updates"
			}
			if kind == "temporal" {
				mode, description = Temporal, "performs temporal operations over"
			}
			// JavaScript array interpolation joins component descriptions without spaces.
			return mode, description + " variables " + strings.Join(entities, ",")
		}
	}
	return PureVal, "doesn't read or update any state variable"
}

// ModeMoreGeneral follows the upstream total qualifier order.
func ModeMoreGeneral(a, b Qualifier) bool {
	rank := func(q Qualifier) int {
		for i, v := range []Qualifier{PureVal, Val, PureDef, Def, Nondet, Action, Temporal, Run} {
			if v == q {
				return i
			}
		}
		return -1
	}
	return rank(a) > rank(b)
}
