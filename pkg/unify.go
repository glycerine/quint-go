package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
)

// TypeUnifier owns the lookup context used by type and row unification.
type TypeUnifier struct {
	Table *insdict.Dict[ID, LookupDefinition]
}

func unifyFailure(location, message string) ([]Substitution, *ErrorTree) {
	e := ErrorLeaf(location, message)
	return nil, &e
}
func wrapUnify(location string, s []Substitution, e *ErrorTree) ([]Substitution, *ErrorTree) {
	if e == nil {
		return s, nil
	}
	v := WrapError(location, *e)
	return nil, &v
}
func bindResult(location string, s []Substitution, e error) ([]Substitution, *ErrorTree) {
	if e != nil {
		return unifyFailure(location, e.Error())
	}
	return s, nil
}

// Unify computes substitutions or a structured incompatibility diagnostic.
// Inputs must be validated types with acyclic aliases and resolved applications.
func (u TypeUnifier) Unify(a, b Type) ([]Substitution, *ErrorTree) {
	loc := "Trying to unify " + TypeString(a) + " and " + TypeString(b)
	if TypeString(a) == TypeString(b) {
		return []Substitution{}, nil
	}
	if a.Kind == "var" {
		s, e := BindTypeVariable(a.Name, b)
		return bindResult(loc, s, e)
	}
	if b.Kind == "var" {
		s, e := BindTypeVariable(b.Name, a)
		return bindResult(loc, s, e)
	}
	if a.Kind == "const" || b.Kind == "const" {
		if a.Kind != "const" {
			a, b = b, a
			loc = "Trying to unify " + TypeString(a) + " and " + TypeString(b)
		}
		var d LookupDefinition
		if a.ID != nil && u.Table != nil {
			d = u.Table.Get(*a.ID)
		}
		if d.Declaration == nil || d.Declaration.Kind != "typedef" {
			return unifyFailure(loc, "Couldn't find type alias "+a.Name)
		}
		if d.Declaration.Type == nil {
			return unifyFailure(loc, "Couldn't unify uninterpreted type "+a.Name+" with different type")
		}
		return u.Unify(*d.Declaration.Type, b)
	}
	if a.Kind == b.Kind {
		switch a.Kind {
		case "set", "list":
			return u.Unify(*a.Elem, *b.Elem)
		case "oper":
			if len(a.Args) != len(b.Args) {
				return unifyFailure(loc, fmt.Sprintf("Expected %d arguments, got %d", len(a.Args), len(b.Args)))
			}
			s, e := u.chain(append(append([]Type{}, a.Args...), *a.Res), append(append([]Type{}, b.Args...), *b.Res))
			return wrapUnify(loc, s, e)
		case "fun":
			s, e := u.Unify(*a.Arg, *b.Arg)
			if e != nil {
				return nil, e
			}
			ar, er := ApplySubstitution(s, *a.Res)
			if er != nil {
				return unifyFailure(loc, er.Error())
			}
			br, er := ApplySubstitution(s, *b.Res)
			if er != nil {
				return unifyFailure(loc, er.Error())
			}
			next, e := u.Unify(ar, br)
			if e != nil {
				return nil, e
			}
			return u.Compose(s, next)
		case "tup", "rec", "sum":
			s, e := u.UnifyRows(*a.Fields, *b.Fields)
			return wrapUnify(loc, s, e)
		}
	}
	return unifyFailure(loc, "Couldn't unify "+a.Kind+" and "+b.Kind)
}
func (u TypeUnifier) chain(a, b []Type) ([]Substitution, *ErrorTree) {
	subs := []Substitution{}
	for i, t := range a {
		x, err := ApplySubstitution(subs, t)
		if err != nil {
			return unifyFailure("applying substitutions", err.Error())
		}
		y, err := ApplySubstitution(subs, b[i])
		if err != nil {
			return unifyFailure("applying substitutions", err.Error())
		}
		next, e := u.Unify(x, y)
		if e != nil {
			return nil, e
		}
		subs, e = u.Compose(next, subs)
		if e != nil {
			return nil, e
		}
	}
	return subs, nil
}

// Compose updates second with first and appends first, preserving upstream
// substitution ordering and unifying duplicate names instead of overwriting.
func (u TypeUnifier) Compose(first, second []Substitution) ([]Substitution, *ErrorTree) {
	out := []Substitution{}
	for _, s := range second {
		var match *Substitution
		for i := range first {
			if first[i].Name == s.Name {
				match = &first[i]
				break
			}
		}
		if match != nil {
			var values []Substitution
			var e *ErrorTree
			if match.Kind != s.Kind {
				return unifyFailure("composing substitutions", "Substitutions with same name ("+s.Name+") but incompatible kinds")
			}
			if s.Kind == "type" {
				values, e = u.Unify(s.Type, match.Type)
			} else {
				values, e = u.UnifyRows(s.Row, match.Row)
			}
			if e != nil {
				return nil, e
			}
			out = append(out, values...)
			continue
		}
		if s.Kind == "type" {
			v, e := ApplySubstitution(first, s.Type)
			if e != nil {
				return unifyFailure("composing substitutions", e.Error())
			}
			s.Type = v
		} else {
			v, e := ApplySubstitution(first, Type{Kind: "rec", Fields: &s.Row})
			if e != nil {
				return unifyFailure("composing substitutions", e.Error())
			}
			s.Row = *v.Fields
		}
		out = append(out, s)
	}
	return append(out, first...), nil
}

// UnifyRows normalizes rows, reconciles shared fields, and joins disjoint open
// tails through a fresh combined tail name, following the upstream algorithm.
func (u TypeUnifier) UnifyRows(a, b Row) ([]Substitution, *ErrorTree) {
	a, b = SimplifyRow(a), SimplifyRow(b)
	loc := "Trying to unify " + RowString(a) + " and " + RowString(b)
	if RowString(a) == RowString(b) {
		return []Substitution{}, nil
	}
	if a.Kind == "var" {
		s, e := BindRowVariable(a.Name, b)
		return bindResult(loc, s, e)
	}
	if b.Kind == "var" {
		s, e := BindRowVariable(b.Name, a)
		return bindResult(loc, s, e)
	}
	if a.Kind != "row" || b.Kind != "row" {
		return unifyFailure(loc, "Couldn't unify "+a.Kind+" and "+b.Kind)
	}
	shared := map[string]bool{}
	left, right := []Type{}, []Type{}
	for _, f := range a.Fields {
		for _, g := range b.Fields {
			if f.Name == g.Name {
				shared[f.Name] = true
				left = append(left, f.Type)
				right = append(right, g.Type)
				break
			}
		}
	}
	if len(left) == 0 {
		if a.Other.Kind == "var" && b.Other.Kind == "var" && a.Other.Name != b.Other.Name {
			tail := Row{Kind: "var", Name: "$" + a.Other.Name + "$" + b.Other.Name}
			ar, br := a, b
			ar.Other = &tail
			br.Other = &tail
			sa, ea := BindRowVariable(a.Other.Name, br)
			if ea != nil {
				return bindResult(loc, sa, ea)
			}
			sb, eb := BindRowVariable(b.Other.Name, ar)
			if eb != nil {
				return bindResult(loc, sb, eb)
			}
			return u.Compose(sa, sb)
		}
		return unifyFailure(loc, "Incompatible tails for rows with disjoint fields: "+RowString(*a.Other)+" and "+RowString(*b.Other))
	}
	ar, br := a, b
	ar.Fields = []RowField{}
	br.Fields = []RowField{}
	for _, f := range a.Fields {
		if !shared[f.Name] {
			ar.Fields = append(ar.Fields, f)
		}
	}
	for _, f := range b.Fields {
		if !shared[f.Name] {
			br.Fields = append(br.Fields, f)
		}
	}
	tails, e := u.UnifyRows(ar, br)
	if e != nil {
		return wrapUnify(loc, nil, e)
	}
	fields, e := u.chain(left, right)
	if e != nil {
		return wrapUnify(loc, nil, e)
	}
	s, e := u.Compose(tails, fields)
	return wrapUnify(loc, s, e)
}
