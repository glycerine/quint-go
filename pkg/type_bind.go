package quint

import "fmt"

// BindTypeVariable constructs one substitution after the type-variable occurs
// check. Equality is handled by the unifier before binding, so binding a variable
// directly to itself is rejected here, matching the upstream helper.
func BindTypeVariable(name string, typ Type) ([]Substitution, error) {
	if err := typ.Validate(); err != nil {
		return nil, err
	}
	names, _, err := TypeNames(&typ)
	if err != nil {
		return nil, err
	}
	for _, n := range names {
		if n == name {
			return nil, fmt.Errorf("Can't bind %s to %s: cyclical binding", name, TypeString(typ))
		}
	}
	copy, err := ApplySubstitution(nil, typ)
	if err != nil {
		return nil, err
	}
	return []Substitution{{Kind: "type", Name: name, Type: copy}}, nil
}

// BindRowVariable checks row tails only. Variables inside field types belong to
// independently checked type structures and do not cause a row-tail occurs error.
func BindRowVariable(name string, row Row) ([]Substitution, error) {
	wrapper := Type{Kind: "rec", Fields: &row}
	if err := wrapper.Validate(); err != nil {
		return nil, err
	}
	for _, n := range RowNames(&row) {
		if n == name {
			return nil, fmt.Errorf("Can't bind %s to %s: cyclical binding", name, RowString(row))
		}
	}
	copy, err := ApplySubstitution(nil, wrapper)
	if err != nil {
		return nil, err
	}
	return []Substitution{{Kind: "row", Name: name, Row: *copy.Fields}}, nil
}
