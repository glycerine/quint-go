package quint

import (
	"fmt"
	"reflect"
	"strings"
)

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

// ModeForEffect classifies operator effects by the state interactions added to
// their parameters. It expects inference to have rejected operator results.
func ModeForEffect(s EffectScheme, annotated Qualifier) (Qualifier, string, error) {
	e := s.Effect
	if e.Kind != "arrow" {
		m, x := ConcreteEffectMode(s)
		return m, x, nil
	}
	if e.Result == nil || e.Result.Kind == "arrow" {
		return "", "", fmt.Errorf("Unexpected arrow found in operator result: %s", EffectString(e))
	}
	message := fmt.Sprintf("has %d parameter", len(e.Params))
	if len(e.Params) > 1 {
		message += "s"
	}
	if e.Result.Kind == "variable" {
		return PureDef, message, nil
	}
	byKind := map[string][]Entity{}
	var concat func(string, Entity)
	concat = func(kind string, e Entity) {
		if e.Kind == "union" {
			for _, x := range e.Entities {
				concat(kind, x)
			}
		} else {
			byKind[kind] = append(byKind[kind], e)
		}
	}
	var gather func(Effect)
	gather = func(e Effect) {
		for _, p := range e.Params {
			if p.Kind == "concrete" {
				for _, c := range p.Components {
					concat(c.Kind, c.Entity)
				}
			}
			if p.Kind == "arrow" {
				gather(p)
				if p.Result.Kind == "concrete" {
					for _, c := range p.Result.Components {
						concat(c.Kind, c.Entity)
					}
				}
			}
		}
	}
	gather(e)
	added := map[string][]Entity{}
	var difference func([]Entity, Entity) []Entity
	difference = func(params []Entity, result Entity) []Entity {
		if result.Kind == "union" {
			out := []Entity{}
			for _, x := range result.Entities {
				out = append(out, difference(params, x)...)
			}
			return out
		}
		if result.Kind == "concrete" {
			// Upstream compares an Entity object with a StateVariable object here;
			// they cannot be equal, so concrete result occurrences are all retained.
			if len(result.StateVariables) == 0 {
				return nil
			}
			return []Entity{result}
		}
		for _, p := range params {
			if reflect.DeepEqual(p, result) {
				return nil
			}
		}
		return []Entity{result}
	}
	for _, c := range e.Result.Components {
		params := append([]Entity{}, byKind[c.Kind]...)
		if c.Kind == "temporal" {
			params = append(params, byKind["update"]...)
		}
		added[c.Kind] = difference(params, c.Entity)
	}
	for _, kind := range []string{"temporal", "update", "read"} {
		synthetic := Effect{Kind: "concrete", Components: []EffectComponent{}}
		for _, entity := range added[kind] {
			synthetic.Components = append(synthetic.Components, EffectComponent{Kind: kind, Entity: entity})
		}
		m, x := ConcreteEffectMode(EffectScheme{Effect: synthetic, EntityVariables: s.EntityVariables})
		if m == PureVal {
			continue
		}
		if annotated == Val && kind == "read" {
			return Def, message, nil
		}
		if m == Val {
			m = Def
		}
		return m, x, nil
	}
	if annotated == Val {
		return Def, message, nil
	}
	return PureDef, message, nil
}
