package quint

// Replacement use already clones owned storage. Preparing substitutions up
// front is necessary only to assign stable identities to bare occurrences,
// which must remain the same across all substitution passes.
func effectSubstitutionsNeedPreparation(subs []EffectSubstitution) bool {
	for _, substitution := range subs {
		switch substitution.Kind {
		case "entity":
			if effectEntityNeedsPreparation(substitution.Entity) {
				return true
			}
		case "effect":
			if effectValueNeedsPreparation(substitution.Effect) {
				return true
			}
		}
	}
	return false
}

func effectEntityNeedsPreparation(entity Entity) bool {
	for _, variable := range entity.StateVariables {
		if variable.identity == nil {
			return true
		}
	}
	for _, child := range entity.Entities {
		if effectEntityNeedsPreparation(child) {
			return true
		}
	}
	return false
}

func effectValueNeedsPreparation(effect Effect) bool {
	for _, component := range effect.Components {
		if effectEntityNeedsPreparation(component.Entity) {
			return true
		}
	}
	for _, parameter := range effect.Params {
		if effectValueNeedsPreparation(parameter) {
			return true
		}
	}
	return effect.Result != nil && effectValueNeedsPreparation(*effect.Result)
}
