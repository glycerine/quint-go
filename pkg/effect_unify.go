package quint

import "fmt"

// UnifyEffects unifies valid effect trees using Quint's restricted effect system.
func UnifyEffects(a, b Effect) ([]EffectSubstitution, *ErrorTree) {
	loc := "Trying to unify " + EffectString(a) + " and " + EffectString(b)
	fail := func(err error) ([]EffectSubstitution, *ErrorTree) { e := ErrorLeaf(loc, err.Error()); return nil, &e }
	wrap := func(s []EffectSubstitution, e *ErrorTree) ([]EffectSubstitution, *ErrorTree) {
		if e != nil {
			v := WrapError(loc, *e)
			return nil, &v
		}
		return s, nil
	}
	a, b = SimplifyEffect(a), SimplifyEffect(b)
	if EffectString(a) == EffectString(b) {
		return []EffectSubstitution{}, nil
	}
	if a.Kind == "variable" {
		s, e := BindEffectVariable(a.Name, b)
		if e != nil {
			return fail(e)
		}
		return s, nil
	}
	if b.Kind == "variable" {
		s, e := BindEffectVariable(b.Name, a)
		if e != nil {
			return fail(e)
		}
		return s, nil
	}
	if a.Kind == "concrete" && b.Kind == "concrete" {
		return wrap(unifyConcreteEffects(a, b))
	}
	if a.Kind == "arrow" && b.Kind == "arrow" {
		if len(a.Params) != len(b.Params) {
			return fail(fmt.Errorf("Expected %d arguments, got %d", len(a.Params), len(b.Params)))
		}
		ar, e := ApplyEffectSubstitution(nil, *a.Result)
		if e != nil {
			return fail(e)
		}
		br, e := ApplyEffectSubstitution(nil, *b.Result)
		if e != nil {
			return fail(e)
		}
		a.Result, b.Result = &ar, &br
		a, sa := simplifyUnificationArrow(a)
		b, sb := simplifyUnificationArrow(b)
		subs, e := ComposeEffectSubstitutions(sa, sb)
		if e != nil {
			return fail(e)
		}
		subs, e = ComposeEffectSubstitutions(subs, nil)
		if e != nil {
			return fail(e)
		}
		pairsA := append(append([]Effect{}, a.Params...), *a.Result)
		pairsB := append(append([]Effect{}, b.Params...), *b.Result)
		for i, x := range pairsA {
			x, e = ApplyEffectSubstitution(subs, x)
			if e != nil {
				return fail(e)
			}
			y, e := ApplyEffectSubstitution(subs, pairsB[i])
			if e != nil {
				return fail(e)
			}
			s, err := UnifyEffects(x, y)
			if err != nil {
				return wrap(nil, err)
			}
			subs, e = ComposeEffectSubstitutions(s, subs)
			if e != nil {
				return fail(e)
			}
		}
		return subs, nil
	}
	return fail(fmt.Errorf("Can't unify different types of effects"))
}

func unifyConcreteEffects(a, b Effect) ([]EffectSubstitution, *ErrorTree) {
	compose := func(a, b []EffectSubstitution) ([]EffectSubstitution, *ErrorTree) {
		s, e := ComposeEffectSubstitutions(a, b)
		if e != nil {
			v := ErrorLeaf("Composing effect substitutions", e.Error())
			return nil, &v
		}
		return s, nil
	}
	general := []EffectSubstitution{}
	var generalErr *ErrorTree
	for _, ca := range a.Components {
		for _, cb := range b.Components {
			if generalErr != nil {
				break
			}
			s := []EffectSubstitution{}
			if ca.Kind == cb.Kind {
				s, generalErr = UnifyEntities(ApplyEntitySubstitution(general, ca.Entity), ApplyEntitySubstitution(general, cb.Entity))
			}
			// All distinct valid component kinds coexist in upstream's compatibility table.
			if generalErr == nil {
				general, generalErr = compose(general, s)
			}
		}
	}
	nullify := func(a, b Effect) ([]EffectSubstitution, *ErrorTree) {
		out := []EffectSubstitution{}
		for _, c := range a.Components {
			matched := false
			for _, d := range b.Components {
				if c.Kind == d.Kind {
					matched = true
					break
				}
			}
			if !matched {
				s, e := UnifyEntities(c.Entity, Entity{Kind: "concrete"})
				if e != nil {
					return nil, e
				}
				out, e = compose(out, s)
				if e != nil {
					return nil, e
				}
			}
		}
		return out, nil
	}
	sa, e := nullify(a, b)
	if e != nil {
		return nil, e
	}
	sb, e := nullify(b, a)
	if e != nil {
		return nil, e
	}
	s, e := compose(sa, sb)
	if e != nil {
		return nil, e
	}
	if generalErr != nil {
		return nil, generalErr
	}
	return compose(s, general)
}

func simplifyUnificationArrow(a Effect) (Effect, []EffectSubstitution) {
	subs := []EffectSubstitution{}
	if len(a.Params) != 1 || a.Params[0].Kind != "concrete" || EffectString(a.Params[0]) != EffectString(*a.Result) {
		return a, subs
	}
	p := Effect{Kind: "concrete", Components: []EffectComponent{}}
	for _, c := range a.Params[0].Components {
		h := hashUnificationEntity(c.Entity)
		p.Components = append(p.Components, EffectComponent{Kind: c.Kind, Entity: h})
		for _, n := range EntityNames(c.Entity) {
			subs = append(subs, EffectSubstitution{Kind: "entity", Name: n, Entity: cloneEntity(h)})
		}
	}
	a.Params = []Effect{p}
	return a, subs
}

func hashUnificationEntity(e Entity) Entity {
	if e.Kind != "union" {
		return cloneEntity(e)
	}
	name := ""
	entities := []Entity{}
	for _, v := range e.Entities {
		v = hashUnificationEntity(v)
		if v.Kind == "variable" {
			if name != "" {
				name += "#"
			}
			name += v.Name
		} else {
			entities = append(entities, v)
		}
	}
	if name == "" {
		if len(entities) == 0 {
			return Entity{Kind: "concrete", StateVariables: []StateVariable{}}
		}
		return Entity{Kind: "union", Entities: entities}
	}
	v := Entity{Kind: "variable", Name: name}
	if len(entities) == 0 {
		return v
	}
	return Entity{Kind: "union", Entities: append([]Entity{v}, entities...)}
}
