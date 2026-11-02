package quint

// EffectFreeNames tracks the ordered free names of one lambda scope.
type EffectFreeNames struct {
	Effects, Entities []string
}

func appendEffectName(names []string, name string) []string {
	for _, n := range names {
		if n == name {
			return names
		}
	}
	return append(names, name)
}

// ExpandEffectFreeNames propagates substitution dependencies in their supplied
// order. The inferrer calls this for every active lambda scope.
func ExpandEffectFreeNames(before EffectFreeNames, subs []EffectSubstitution) EffectFreeNames {
	out := EffectFreeNames{Effects: append([]string{}, before.Effects...), Entities: append([]string{}, before.Entities...)}
	contains := func(names []string, n string) bool {
		for _, v := range names {
			if v == n {
				return true
			}
		}
		return false
	}
	for _, s := range subs {
		if s.Kind == "effect" && contains(out.Effects, s.Name) {
			es, vs := EffectNames(s.Effect)
			for _, n := range es {
				out.Effects = appendEffectName(out.Effects, n)
			}
			for _, n := range vs {
				out.Entities = appendEffectName(out.Entities, n)
			}
		}
		if s.Kind == "entity" && contains(out.Entities, s.Name) {
			for _, n := range EntityNames(s.Entity) {
				out.Entities = appendEffectName(out.Entities, n)
			}
		}
	}
	return out
}

// GeneralizeEffect quantifies names absent from the current lambda's free set.
func GeneralizeEffect(effect Effect, free EffectFreeNames) EffectScheme {
	es, vs := EffectNames(effect)
	difference := func(names, exclude []string) []string {
		out := []string{}
		for _, n := range names {
			found := false
			for _, x := range exclude {
				if x == n {
					found = true
					break
				}
			}
			if !found {
				out = append(out, n)
			}
		}
		return out
	}
	return EffectScheme{Effect: cloneEffectSubstitutionValue(effect), EffectVariables: difference(es, free.Effects), EntityVariables: difference(vs, free.Entities)}
}
