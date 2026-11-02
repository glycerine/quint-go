package quint

import "github.com/glycerine/insdict"

// ConvertInit converts q::init and its dependencies to predicates. The input
// must have resolved names; recursive definitions have already been rejected.
func ConvertInit(module Module, table *insdict.Dict[ID, LookupDefinition], modes *insdict.Dict[string, Qualifier]) (Module, []Diagnostic) {
	names := map[string]bool{"q::init": true}
	active := map[string]bool{}
	var find func(Expr)
	var dependency func(string, ID)
	dependency = func(name string, id ID) {
		d, ok := table.Get2(id)
		if !ok || d.Declaration == nil || d.Declaration.Kind != "def" {
			return
		}
		names[name] = true
		key := d.ID.String()
		if active[key] {
			return
		}
		active[key] = true
		find(*d.Declaration.Expr)
		delete(active, key)
	}
	find = func(e Expr) {
		if e.Kind == "name" {
			dependency(e.Name, e.ID)
		}
		if e.Kind == "app" {
			dependency(e.Opcode, e.ID)
		}
		if e.Opdef != nil {
			find(*e.Opdef.Expr)
		}
		for _, a := range e.Args {
			find(a)
		}
		if e.Body != nil {
			find(*e.Body)
		}
	}
	for _, d := range module.Declarations {
		if d.Kind == "def" && names[d.Name] {
			find(*d.Expr)
		}
	}
	errors := []Diagnostic{}
	check := func(id ID, name string, inside bool) {
		if inside || !names[name] {
			return
		}
		d, ok := table.Get2(id)
		if !ok {
			return
		}
		action := false
		mode, known := Qualifier(""), false
		if modes != nil {
			mode, known = modes.Get2(d.ID.String())
		}
		if known {
			action = mode == Action
		} else {
			action = d.Declaration != nil && d.Declaration.Kind == "def" && d.Declaration.Qualifier == Action
		}
		if action {
			errors = append(errors, Diagnostic{Code: "QNT409", Message: "Action " + name + " is used both for init and for step, and therefore can't be converted into TLA+. You can duplicate this with a different name to use on init. Sorry Quint can't do it for you yet.", Reference: &id})
		}
	}
	var transform func(Expr, bool) Expr
	transform = func(e Expr, inside bool) Expr {
		out := e
		if e.Kind == "name" {
			check(e.ID, e.Name, inside)
		}
		if e.Kind == "app" {
			check(e.ID, e.Opcode, inside)
			if inside && e.Opcode == "assign" {
				out.Opcode = "eq"
			}
		}
		out.Args = make([]Expr, len(e.Args))
		for n, a := range e.Args {
			out.Args[n] = transform(a, inside)
		}
		if e.Opdef != nil {
			d := *e.Opdef
			body := transform(*d.Expr, inside || names[d.Name])
			d.Expr = &body
			out.Opdef = &d
		}
		if e.Body != nil {
			body := transform(*e.Body, inside)
			out.Body = &body
		}
		return out
	}
	out := module
	out.Declarations = append([]Declaration{}, module.Declarations...)
	for n, d := range out.Declarations {
		inside := d.Kind == "def" && names[d.Name]
		if d.Expr != nil {
			e := transform(*d.Expr, inside)
			d.Expr = &e
		}
		if d.Assumption != nil {
			e := transform(*d.Assumption, inside)
			d.Assumption = &e
		}
		d.Overrides = append([]Override{}, d.Overrides...)
		for j, o := range d.Overrides {
			d.Overrides[j].Expr = transform(o.Expr, inside)
		}
		out.Declarations[n] = d
	}
	if len(errors) > 0 {
		return Module{}, errors
	}
	return out, errors
}
