package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
	"strings"
)

// TypeApplicationResolver expands parameterized aliases in type trees.
// Errors contains recoverable arity errors keyed by application ID; malformed
// references and recursive applications are returned as Go errors.
type TypeApplicationResolver struct {
	Table  *insdict.Dict[ID, LookupDefinition]
	Errors *insdict.Dict[string, ErrorTree]
	fresh  FreshNames
}

func NewTypeApplicationResolver(table *insdict.Dict[ID, LookupDefinition]) *TypeApplicationResolver {
	return &TypeApplicationResolver{Table: table, Errors: insdict.NewDict[string, ErrorTree]()}
}
func (r *TypeApplicationResolver) Resolve(input Type) (Type, error) {
	active := map[ID]bool{}
	var visit func(Type) (Type, error)
	visit = func(t Type) (Type, error) {
		out, err := ApplySubstitution(nil, t)
		if err != nil {
			return Type{}, err
		}
		child := func(p *Type) error {
			if p == nil {
				return nil
			}
			v, e := visit(*p)
			if e == nil {
				*p = v
			}
			return e
		}
		for _, p := range []*Type{out.Elem, out.Arg, out.Res} {
			if err = child(p); err != nil {
				return Type{}, err
			}
		}
		for i := range out.Args {
			if err = child(&out.Args[i]); err != nil {
				return Type{}, err
			}
		}
		var rows func(*Row) error
		rows = func(row *Row) error {
			if row == nil {
				return nil
			}
			for i := range row.Fields {
				if e := child(&row.Fields[i].Type); e != nil {
					return e
				}
			}
			return rows(row.Other)
		}
		if err = rows(out.Fields); err != nil {
			return Type{}, err
		}
		if out.Kind != "app" {
			return out, nil
		}
		if out.ID == nil || out.Ctor == nil || out.Ctor.ID == nil {
			return Type{}, fmt.Errorf("type application requires application and constructor IDs")
		}
		def, ok := r.Table.Get2(*out.Ctor.ID)
		if !ok || def.Declaration == nil || def.Declaration.Kind != "typedef" || def.Declaration.Type == nil {
			return Type{}, fmt.Errorf("type constructor %s has no alias definition", out.Ctor.Name)
		}
		d := def.Declaration
		body, err := ApplySubstitution(nil, *d.Type)
		if err != nil {
			return Type{}, err
		}
		names := insdict.NewDict[string, string]()
		for _, p := range d.Params {
			fresh := r.fresh.Next(p)
			names.Set(p, fresh)
		}
		// Upstream constructs a Map from all parameters: duplicates consume
		// fresh names, retain their first position, and keep their last value.
		params := []string{}
		for _, fresh := range names.All() {
			params = append(params, fresh)
		}
		// Rename both variable sorts while retaining annotation IDs.
		_ = WalkType(&body, func(t *Type) (bool, error) {
			if t.Kind == "var" {
				if n, ok := names.Get2(t.Name); ok {
					t.Name = n
				}
			}
			var renameRow func(*Row)
			renameRow = func(row *Row) {
				if row == nil {
					return
				}
				if row.Kind == "var" {
					if n, ok := names.Get2(row.Name); ok {
						row.Name = n
					}
				}
				renameRow(row.Other)
			}
			renameRow(t.Fields)
			return true, nil
		})
		if len(params) != len(out.Args) {
			args := []string{}
			for _, a := range out.Args {
				args = append(args, TypeString(a))
			}
			amount := "many"
			if len(params) > len(out.Args) {
				amount = "few"
			}
			ctor := TypeString(*out.Ctor)
			r.Errors.Set(out.ID.String(), ErrorLeaf("applying type constructor "+ctor+" to arguments "+strings.Join(args, ", "), fmt.Sprintf("too %s arguments supplied: %s only accepts %d parameters", amount, ctor, len(params))))
			return out, nil
		}
		if active[def.ID] {
			return Type{}, fmt.Errorf("cyclic type application %s", d.Name)
		}
		subs := []Substitution{}
		for i, p := range params {
			arg, e := ResolveTypeAlias(r.Table, out.Args[i])
			if e != nil {
				return Type{}, e
			}
			subs = append(subs, Substitution{Kind: "type", Name: p, Type: arg})
			if arg.Kind == "rec" || arg.Kind == "tup" {
				subs = append(subs, Substitution{Kind: "row", Name: p, Row: *arg.Fields})
			}
			if arg.Kind == "var" {
				subs = append(subs, Substitution{Kind: "row", Name: p, Row: Row{Kind: "var", Name: arg.Name}})
			}
		}
		body, err = ApplySubstitution(subs, body)
		if err != nil {
			return Type{}, err
		}
		active[def.ID] = true
		result, err := visit(body)
		delete(active, def.ID)
		return result, err
	}
	return visit(input)
}

// ResolveLookupTable replaces lookup payloads in insertion order, as the upstream
// resolver constructor does. Previously resolved entries are visible to later
// entries. On structural failure, earlier replacements remain in the table.
func (r *TypeApplicationResolver) ResolveLookupTable() error {
	for id, def := range r.Table.All() {
		if def.Declaration != nil {
			d, err := TransformDeclarationTypes(*def.Declaration, r.Resolve)
			if err != nil {
				return err
			}
			def.Declaration = &d
			def.identity = new(byte)
		} else if def.Parameter != nil && def.Parameter.TypeAnnotation != nil {
			p := *def.Parameter
			t, err := r.Resolve(*p.TypeAnnotation)
			if err != nil {
				return err
			}
			p.TypeAnnotation = &t
			def.Parameter = &p
			def.identity = new(byte)
		}
		r.Table.Set(id, def)
	}
	return nil
}
