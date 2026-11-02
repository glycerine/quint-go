package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
)

// ResolveTypeAlias follows outer alias references. Abstract types and unresolved
// references remain unchanged. The returned tree is independent of the input.
func ResolveTypeAlias(table *insdict.Dict[ID, LookupDefinition], t Type) (Type, error) {
	return resolveTypeAliases(table, t, false)
}

// InlineTypeAliasesInType recursively expands aliases in type children and rows.
// Application constructors stay nominal: type application resolution handles
// their parameters separately. Cyclic aliases return an error.
func InlineTypeAliasesInType(table *insdict.Dict[ID, LookupDefinition], t Type) (Type, error) {
	return resolveTypeAliases(table, t, true)
}

func resolveTypeAliases(table *insdict.Dict[ID, LookupDefinition], input Type, recursive bool) (Type, error) {
	active := map[ID]bool{}
	var typ func(Type) (Type, error)
	var row func(*Row) error
	typ = func(t Type) (Type, error) {
		if t.Kind == "const" && t.ID != nil {
			def, ok := table.Get2(*t.ID)
			if ok && def.Declaration != nil && def.Declaration.Kind == "typedef" && def.Declaration.Type != nil {
				if active[def.ID] {
					return Type{}, fmt.Errorf("cyclic type alias %s", def.Name)
				}
				active[def.ID] = true
				result, err := typ(*def.Declaration.Type)
				delete(active, def.ID)
				return result, err
			}
		}
		out, err := ApplySubstitution(nil, t)
		if err != nil || !recursive {
			return out, err
		}
		for _, child := range []*Type{out.Elem, out.Arg, out.Res} {
			if child != nil {
				value, err := typ(*child)
				if err != nil {
					return Type{}, err
				}
				*child = value
			}
		}
		for i := range out.Args {
			value, err := typ(out.Args[i])
			if err != nil {
				return Type{}, err
			}
			out.Args[i] = value
		}
		if err := row(out.Fields); err != nil {
			return Type{}, err
		}
		return out, nil
	}
	row = func(r *Row) error {
		if r == nil {
			return nil
		}
		for i := range r.Fields {
			value, err := typ(r.Fields[i].Type)
			if err != nil {
				return err
			}
			r.Fields[i].Type = value
		}
		return row(r.Other)
	}
	return typ(input)
}
