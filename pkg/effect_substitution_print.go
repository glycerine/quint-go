package quint

import (
	"fmt"
	"strings"
)

// EffectSubstitutionsString renders an ordered substitution list using Quint's
// effect diagnostics notation. Repeated names and empty entities are retained;
// commas between substitutions have no added spaces. The input is borrowed and
// never modified. Invalid kinds, missing arrow results and cyclic trees return
// errors instead of reaching the unchecked effect/entity printers.
func EffectSubstitutionsString(substitutions []EffectSubstitution) (string, error) {
	parts := make([]string, len(substitutions))
	validation := effectSubstitutionPrintValidation{}
	for i := range substitutions {
		substitution := &substitutions[i]
		var value string
		var err error
		switch substitution.Kind {
		case "effect":
			if err = validation.effect(&substitution.Effect); err == nil {
				value = EffectString(substitution.Effect)
			}
		case "entity":
			if err = validation.entity(&substitution.Entity); err == nil {
				value = EntityString(substitution.Entity)
			}
		default:
			err = fmt.Errorf("unknown effect substitution kind %q", substitution.Kind)
		}
		if err != nil {
			return "", fmt.Errorf("substitution %d: %w", i, err)
		}
		parts[i] = substitution.Name + " |-> " + value
	}
	return "[" + strings.Join(parts, ",") + "]", nil
}

type effectSubstitutionPrintValidation struct {
	effects  map[*Effect]bool
	entities map[*Entity]bool
}

func (v *effectSubstitutionPrintValidation) effect(effect *Effect) error {
	if effect == nil {
		return fmt.Errorf("arrow effect missing result")
	}
	if v.effects[effect] {
		return fmt.Errorf("cyclic effect tree")
	}
	if v.effects == nil {
		v.effects = map[*Effect]bool{}
	}
	v.effects[effect] = true
	defer delete(v.effects, effect)
	switch effect.Kind {
	case "variable":
		if effect.Name == "" {
			return fmt.Errorf("effect variable missing name")
		}
	case "concrete":
		for i := range effect.Components {
			component := &effect.Components[i]
			switch component.Kind {
			case "read", "update", "temporal":
			default:
				return fmt.Errorf("unknown effect component %q", component.Kind)
			}
			if err := v.entity(&component.Entity); err != nil {
				return err
			}
		}
	case "arrow":
		for i := range effect.Params {
			if err := v.effect(&effect.Params[i]); err != nil {
				return err
			}
		}
		return v.effect(effect.Result)
	default:
		return fmt.Errorf("unknown effect kind %q", effect.Kind)
	}
	return nil
}

func (v *effectSubstitutionPrintValidation) entity(entity *Entity) error {
	if v.entities[entity] {
		return fmt.Errorf("cyclic entity tree")
	}
	if v.entities == nil {
		v.entities = map[*Entity]bool{}
	}
	v.entities[entity] = true
	defer delete(v.entities, entity)
	switch entity.Kind {
	case "variable":
		if entity.Name == "" {
			return fmt.Errorf("entity variable missing name")
		}
	case "concrete":
	case "union":
		for i := range entity.Entities {
			if err := v.entity(&entity.Entities[i]); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unknown entity kind %q", entity.Kind)
	}
	return nil
}
