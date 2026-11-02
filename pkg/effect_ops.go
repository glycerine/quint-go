package quint

import "strings"

func EffectString(e Effect) string {
	switch e.Kind {
	case "variable":
		return e.Name
	case "concrete":
		if len(e.Components) == 0 {
			return "Pure"
		}
		parts := make([]string, len(e.Components))
		for i, c := range e.Components {
			parts[i] = EffectComponentString(c)
		}
		return strings.Join(parts, " & ")
	case "arrow":
		parts := make([]string, len(e.Params))
		for i, p := range e.Params {
			parts[i] = EffectString(p)
		}
		return "(" + strings.Join(parts, ", ") + ") => " + EffectString(*e.Result)
	}
	return "<invalid effect>"
}
func EffectComponentString(c EffectComponent) string {
	name := ""
	switch c.Kind {
	case "read":
		name = "Read"
	case "update":
		name = "Update"
	case "temporal":
		name = "Temporal"
	default:
		return "<invalid component>"
	}
	return name + "[" + EntityString(c.Entity) + "]"
}
func EntityString(e Entity) string {
	switch e.Kind {
	case "variable":
		return e.Name
	case "concrete":
		parts := make([]string, len(e.StateVariables))
		for i, v := range e.StateVariables {
			parts[i] = "'" + v.Name + "'"
		}
		return strings.Join(parts, ", ")
	case "union":
		parts := make([]string, len(e.Entities))
		for i, v := range e.Entities {
			parts[i] = EntityString(v)
		}
		return strings.Join(parts, ", ")
	}
	return "<invalid entity>"
}

// EntityNames preserves repeated variable occurrences, as the upstream helper
// does; EffectNames deduplicates them when building a quantified-name set.
func EntityNames(e Entity) []string {
	out := []string{}
	switch e.Kind {
	case "variable":
		out = append(out, e.Name)
	case "union":
		for _, child := range e.Entities {
			out = append(out, EntityNames(child)...)
		}
	}
	return out
}
func EffectNames(e Effect) (effects, entities []string) {
	effects = []string{}
	entities = []string{}
	seenE, seenV := map[string]bool{}, map[string]bool{}
	var visit func(Effect)
	visit = func(e Effect) {
		switch e.Kind {
		case "variable":
			if !seenE[e.Name] {
				seenE[e.Name] = true
				effects = append(effects, e.Name)
			}
		case "concrete":
			for _, c := range e.Components {
				for _, name := range EntityNames(c.Entity) {
					if !seenV[name] {
						seenV[name] = true
						entities = append(entities, name)
					}
				}
			}
		case "arrow":
			for _, p := range e.Params {
				visit(p)
			}
			if e.Result != nil {
				visit(*e.Result)
			}
		}
	}
	visit(e)
	return
}
