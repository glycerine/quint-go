package quint

import (
	"fmt"
	"reflect"
)

type Substitution struct {
	Kind string
	Name string
	Type Type
	Row  Row
}

// ApplySubstitution resolves replacement chains, preserving the ordered lookup
// semantics of upstream. Cyclic replacements return an error rather than recurse
// forever. Inputs and replacement values are never mutated.
func ApplySubstitution(subs []Substitution, t Type) (Type, error) {
	active := map[string]bool{}
	var typ func(Type) (Type, error)
	var row func(Row) (Row, error)
	typ = func(t Type) (Type, error) {
		if t.Kind == "var" {
			for _, s := range subs {
				if s.Name != t.Name {
					continue
				}
				if s.Kind != "type" {
					break
				}
				if reflect.DeepEqual(s.Type, t) {
					return t, nil
				}
				key := "type:" + t.Name
				if active[key] {
					return Type{}, fmt.Errorf("cyclic substitution for %s", t.Name)
				}
				active[key] = true
				v, err := typ(s.Type)
				delete(active, key)
				return v, err
			}
		}
		out := t
		if t.ID != nil {
			id := *t.ID
			out.ID = &id
		}
		cp := func(p *Type) (*Type, error) {
			if p == nil {
				return nil, nil
			}
			v, err := typ(*p)
			return &v, err
		}
		var err error
		if out.Elem, err = cp(t.Elem); err != nil {
			return Type{}, err
		}
		if out.Arg, err = cp(t.Arg); err != nil {
			return Type{}, err
		}
		if out.Res, err = cp(t.Res); err != nil {
			return Type{}, err
		}
		if t.Ctor != nil {
			v := *t.Ctor
			out.Ctor = &v
		}
		if t.Args != nil {
			out.Args = make([]Type, len(t.Args))
			for i, a := range t.Args {
				out.Args[i], err = typ(a)
				if err != nil {
					return Type{}, err
				}
			}
		}
		if t.Fields != nil {
			v, err := row(*t.Fields)
			if err != nil {
				return Type{}, err
			}
			out.Fields = &v
		}
		return out, nil
	}
	row = func(r Row) (Row, error) {
		if r.Kind == "var" {
			for _, s := range subs {
				if s.Name != r.Name || s.Kind != "row" {
					continue
				}
				if reflect.DeepEqual(s.Row, r) {
					return r, nil
				}
				key := "row:" + r.Name
				if active[key] {
					return Row{}, fmt.Errorf("cyclic row substitution for %s", r.Name)
				}
				active[key] = true
				v, err := row(s.Row)
				delete(active, key)
				return v, err
			}
		}
		out := r
		if r.Fields != nil {
			out.Fields = make([]RowField, len(r.Fields))
			for i, f := range r.Fields {
				v, err := typ(f.Type)
				if err != nil {
					return Row{}, err
				}
				out.Fields[i] = RowField{Name: f.Name, Type: v}
			}
		}
		if r.Other != nil {
			v, err := row(*r.Other)
			if err != nil {
				return Row{}, err
			}
			out.Other = &v
		}
		return out, nil
	}
	if err := t.Validate(); err != nil {
		return Type{}, err
	}
	result, err := typ(t)
	if err != nil {
		return Type{}, err
	}
	if err = result.Validate(); err != nil {
		return Type{}, err
	}
	return result, nil
}
