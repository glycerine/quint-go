package quint

import "github.com/glycerine/insdict"

// UnshadowNames renames every lambda parameter and shadowing let definition
// using its enclosing expression ID, matching the upstream Apalache preparation
// pass. It returns a new expression tree and updates reference entries in table.
// Types and other unchanged metadata are shared and must be treated as immutable.
// The input must have globally unique IDs and a completed name lookup table.
func UnshadowNames(module Module, table *insdict.Dict[ID, LookupDefinition]) Module {
	names := insdict.NewDict[string, string]()
	var transform func(*Expr) *Expr
	transform = func(input *Expr) *Expr {
		if input == nil {
			return nil
		}
		e := *input
		switch e.Kind {
		case "lambda":
			e.Params = append([]Parameter(nil), e.Params...)
			for i := range e.Params {
				p := &e.Params[i]
				p.Name += "_" + e.ID.String()
				names.Set(p.ID.String(), p.Name)
			}
		case "let":
			if e.Opdef != nil {
				def := *e.Opdef
				if table.Get(def.ID).Shadowing {
					def.Name += "_" + e.ID.String()
					names.Set(def.ID.String(), def.Name)
				}
				e.Opdef = &def
			}
		case "name", "app":
			def := table.Get(e.ID)
			if name := names.Get(def.ID.String()); name != "" {
				def.Name = name
				def.identity = new(byte)
				// The lookup payload mirrors the renamed name without changing
				// the original binding or other references to that payload.
				if def.Parameter != nil {
					p := *def.Parameter
					p.Name = name
					def.Parameter = &p
				}
				if def.Declaration != nil {
					d := *def.Declaration
					d.Name = name
					def.Declaration = &d
				}
				table.Set(e.ID, def)
				if e.Kind == "name" {
					e.Name = name
				} else {
					e.Opcode = name
				}
			}
		}
		if e.Args != nil {
			e.Args = append([]Expr{}, e.Args...)
			for i := range e.Args {
				e.Args[i] = *transform(&e.Args[i])
			}
		}
		if e.Opdef != nil {
			e.Opdef.Expr = transform(e.Opdef.Expr)
		}
		e.Body = transform(e.Body)
		return &e
	}
	module.Declarations = append([]Declaration(nil), module.Declarations...)
	for i := range module.Declarations {
		d := &module.Declarations[i]
		d.Expr = transform(d.Expr)
		d.Assumption = transform(d.Assumption)
		if d.Overrides != nil {
			d.Overrides = append([]Override{}, d.Overrides...)
			for j := range d.Overrides {
				d.Overrides[j].Expr = *transform(&d.Overrides[j].Expr)
			}
		}
	}
	return module
}
