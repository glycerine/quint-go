package quint

import (
	"strconv"
	"strings"
)

// CanonicalTypeScheme renames quantified variables by quantifier order for
// diagnostics, preserving free variables and the input tree.
func CanonicalTypeScheme(s TypeScheme) ([]string, Type, *ErrorTree) {
	names := []string{}
	ts, rs := []Substitution{}, []Substitution{}
	for i, n := range s.TypeVariables {
		fresh := "t" + strconv.Itoa(i)
		names = append(names, fresh)
		ts = append(ts, Substitution{Kind: "type", Name: n, Type: Type{Kind: "var", Name: fresh}})
	}
	for i, n := range s.RowVariables {
		fresh := "r" + strconv.Itoa(i)
		names = append(names, fresh)
		rs = append(rs, Substitution{Kind: "row", Name: n, Row: Row{Kind: "var", Name: fresh}})
	}
	subs, e := (TypeUnifier{}).Compose(ts, rs)
	if e != nil {
		return nil, Type{}, e
	}
	typ, err := ApplySubstitution(subs, s.Type)
	if err != nil {
		e := ErrorLeaf("canonicalizing type scheme", err.Error())
		return nil, Type{}, &e
	}
	return names, typ, nil
}
func TypeSchemeString(s TypeScheme) (string, *ErrorTree) {
	names, typ, e := CanonicalTypeScheme(s)
	if e != nil {
		return "", e
	}
	prefix := ""
	if len(names) > 0 {
		prefix = "∀ " + strings.Join(names, ", ") + " . "
	}
	return prefix + TypeString(typ), nil
}
