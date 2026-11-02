package quint

import (
	"reflect"
	"strings"
)

// UnifyEntities implements Quint's restricted set unification, preserving
// substitution order and source references. Inputs must be valid entities.
func UnifyEntities(a, b Entity) ([]EffectSubstitution, *ErrorTree) {
	a, b = FlattenEntityUnions(a), FlattenEntityUnions(b)
	location := "Trying to unify entities [" + EntityString(a) + "] and [" + EntityString(b) + "]"
	fail := func(message string) ([]EffectSubstitution, *ErrorTree) {
		e := ErrorLeaf(location, message)
		return nil, &e
	}
	names := func(vs []StateVariable) string {
		ns := make([]string, len(vs))
		for i, v := range vs {
			ns[i] = v.Name
		}
		return strings.Join(ns, ",")
	}
	contains := func(vs []StateVariable, name string) bool {
		for _, v := range vs {
			if v.Name == name {
				return true
			}
		}
		return false
	}
	if a.Kind == "concrete" && b.Kind == "concrete" {
		equal := true
		for _, v := range a.StateVariables {
			equal = equal && contains(b.StateVariables, v.Name)
		}
		for _, v := range b.StateVariables {
			equal = equal && contains(a.StateVariables, v.Name)
		}
		if equal {
			return []EffectSubstitution{}, nil
		}
		return fail("Expected [" + names(a.StateVariables) + "] and [" + names(b.StateVariables) + "] to be the same")
	}
	if a.Kind == "variable" && b.Kind == "variable" && a.Name == b.Name {
		return []EffectSubstitution{}, nil
	}
	if a.Kind == "variable" {
		return BindEntityVariable(a.Name, b), nil
	}
	if b.Kind == "variable" {
		return BindEntityVariable(b.Name, a), nil
	}
	if reflect.DeepEqual(a, b) {
		return []EffectSubstitution{}, nil
	}
	if a.Kind == "concrete" && b.Kind == "union" {
		return UnifyEntities(b, a)
	}
	if a.Kind == "union" && b.Kind == "concrete" {
		covered := []StateVariable{}
		for _, e := range a.Entities {
			if e.Kind == "concrete" {
				covered = append(covered, e.StateVariables...)
			}
		}
		uncovered := []StateVariable{}
		for _, v := range covered {
			if !contains(b.StateVariables, v.Name) {
				uncovered = append(uncovered, v)
			}
		}
		if len(uncovered) > 0 {
			return fail("Expected [" + names(uncovered) + "] to be in [" + names(b.StateVariables) + "]")
		}
		remaining := Entity{Kind: "concrete", StateVariables: []StateVariable{}}
		for _, v := range b.StateVariables {
			if !contains(covered, v.Name) {
				remaining.StateVariables = append(remaining.StateVariables, v)
			}
		}
		out := []EffectSubstitution{}
		errors := []ErrorTree{}
		for _, e := range a.Entities {
			if e.Kind == "concrete" {
				continue
			}
			subs, err := UnifyEntities(e, remaining)
			if err != nil {
				errors = append(errors, *err)
			} else {
				out = append(out, subs...)
			}
		}
		if len(errors) > 0 {
			return nil, &ErrorTree{Location: location, Children: errors}
		}
		return out, nil
	}
	if a.Kind == "union" && b.Kind == "union" {
		common := []Entity{}
		for _, x := range a.Entities {
			for _, y := range b.Entities {
				if reflect.DeepEqual(x, y) {
					common = append(common, x)
					break
				}
			}
		}
		if len(common) > 0 {
			difference := func(e Entity) Entity {
				out := Entity{Kind: "union", Entities: []Entity{}}
				for _, x := range e.Entities {
					found := false
					for _, y := range common {
						if reflect.DeepEqual(x, y) {
							found = true
							break
						}
					}
					if !found {
						out.Entities = append(out.Entities, x)
					}
				}
				return out
			}
			return UnifyEntities(difference(a), difference(b))
		}
		return fail("Unification for unions of entities is not implemented")
	}
	return fail("Invalid entity kinds")
}
