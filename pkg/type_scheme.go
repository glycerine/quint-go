package quint

// FreeSchemeVariables returns variables not quantified by the scheme, preserving
// type-tree encounter order independently for type and row variables.
func FreeSchemeVariables(s TypeScheme) (types, rows []string, err error) {
	types, rows, err = TypeNames(&s.Type)
	if err != nil {
		return nil, nil, err
	}
	difference := func(all, bound []string) []string {
		found := map[string]bool{}
		for _, n := range bound {
			found[n] = true
		}
		out := []string{}
		for _, n := range all {
			if !found[n] {
				out = append(out, n)
			}
		}
		return out
	}
	return difference(types, s.TypeVariables), difference(rows, s.RowVariables), nil
}

// InstantiateScheme replaces quantified variables with fresh _t variables,
// allocating type variables before row variables. Share fresh across inference.
func (u TypeUnifier) InstantiateScheme(s TypeScheme, fresh *FreshNames) (Type, *ErrorTree) {
	ts, rs := []Substitution{}, []Substitution{}
	for _, n := range s.TypeVariables {
		ts = append(ts, Substitution{Kind: "type", Name: n, Type: Type{Kind: "var", Name: fresh.Next("_t")}})
	}
	for _, n := range s.RowVariables {
		rs = append(rs, Substitution{Kind: "row", Name: n, Row: Row{Kind: "var", Name: fresh.Next("_t")}})
	}
	subs, e := u.Compose(ts, rs)
	if e != nil {
		return Type{}, e
	}
	typ, err := ApplySubstitution(subs, s.Type)
	if err != nil {
		v := ErrorLeaf("instantiating type scheme", err.Error())
		return Type{}, &v
	}
	return typ, nil
}

// CheckAnnotationGenerality rejects concrete bindings of variables that occur
// in a declared polymorphic annotation, retaining substitution order by sort.
func CheckAnnotationGenerality(subs []Substitution, annotation Type) *ErrorTree {
	types, rows, err := TypeNames(&annotation)
	if err != nil {
		e := ErrorLeaf("Checking type annotation "+TypeString(annotation), err.Error())
		return &e
	}
	contains := func(names []string, n string) bool {
		for _, v := range names {
			if v == n {
				return true
			}
		}
		return false
	}
	children := []ErrorTree{}
	for _, kind := range []string{"type", "row"} {
		for _, s := range subs {
			if s.Kind != kind {
				continue
			}
			expected := ""
			if kind == "type" && s.Type.Kind != "var" && contains(types, s.Name) {
				expected = TypeString(s.Type)
			}
			if kind == "row" && s.Row.Kind != "var" && contains(rows, s.Name) {
				expected = RowString(s.Row)
			}
			if expected != "" {
				children = append(children, ErrorLeaf("Checking variable "+s.Name, "Type annotation is too general: "+s.Name+" should be "+expected))
			}
		}
	}
	if len(children) == 0 {
		return nil
	}
	return &ErrorTree{Location: "Checking type annotation " + TypeString(annotation), Children: children}
}
