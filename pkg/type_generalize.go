package quint

// TypeVariables keeps independent, encounter-ordered type and row namespaces.
type TypeVariables struct{ Types, Rows []string }

func orderedVariableUnion(first, second []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, group := range [][]string{first, second} {
		for _, n := range group {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// FreeVariablesInScope follows upstream scope ordering: each later scheme's
// variables precede the previously accumulated variables.
func FreeVariablesInScope(schemes []TypeScheme) (TypeVariables, error) {
	result := TypeVariables{Types: []string{}, Rows: []string{}}
	for _, scheme := range schemes {
		types, rows, err := FreeSchemeVariables(scheme)
		if err != nil {
			return TypeVariables{}, err
		}
		result.Types = orderedVariableUnion(types, result.Types)
		result.Rows = orderedVariableUnion(rows, result.Rows)
	}
	return result, nil
}

// ExpandFreeVariables adds variables introduced by bindings of already-free
// names. Substitutions must be in dependency order. Original names are retained.
func ExpandFreeVariables(free TypeVariables, subs []Substitution) (TypeVariables, error) {
	out := TypeVariables{Types: orderedVariableUnion(free.Types, nil), Rows: orderedVariableUnion(free.Rows, nil)}
	has := func(names []string, n string) bool {
		for _, v := range names {
			if v == n {
				return true
			}
		}
		return false
	}
	for _, s := range subs {
		if s.Kind == "type" && has(out.Types, s.Name) {
			types, rows, err := TypeNames(&s.Type)
			if err != nil {
				return TypeVariables{}, err
			}
			out.Types = orderedVariableUnion(out.Types, types)
			out.Rows = orderedVariableUnion(out.Rows, rows)
		}
		if s.Kind == "row" && has(out.Rows, s.Name) {
			out.Rows = orderedVariableUnion(out.Rows, RowNames(&s.Row))
		}
	}
	return out, nil
}

// GeneralizeType quantifies variables free in the completed scope but not free
// before the definition. The caller expands before with solved bindings first.
func GeneralizeType(typ Type, after, before TypeVariables) (TypeScheme, error) {
	difference := func(all, excluded []string) []string {
		seen := map[string]bool{}
		for _, n := range excluded {
			seen[n] = true
		}
		out := []string{}
		for _, n := range all {
			if !seen[n] {
				out = append(out, n)
				seen[n] = true
			}
		}
		return out
	}
	copy, err := ApplySubstitution(nil, typ)
	if err != nil {
		return TypeScheme{}, err
	}
	return TypeScheme{Type: copy, TypeVariables: difference(after.Types, before.Types), RowVariables: difference(after.Rows, before.Rows)}, nil
}
