package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
	"strings"
)

// FlattenInstances creates instance modules followed by the rewritten owner.
// Imports/exports should already have been expanded. Callers register generated
// modules and rerun name resolution before subsequent flattening.
func FlattenInstances(module Module, modules *insdict.Dict[string, Module], table *insdict.Dict[ID, LookupDefinition], ids *IDGenerator, sources SourceMap, analysis AnalysisOutput) ([]Module, error) {
	var rewrite func(Expr) Expr
	rewrite = func(e Expr) Expr {
		out := e
		if e.Kind == "name" || e.Kind == "app" {
			if d, ok := table.Get2(e.ID); ok && d.ImportedFrom != nil && d.ImportedFrom.Kind == "instance" {
				parts := []string{}
				for j := len(d.Namespaces) - 1; j >= 0; j-- {
					if d.Namespaces[j] != "" {
						parts = append(parts, d.Namespaces[j])
					}
				}
				if d.Name != "" {
					parts = append(parts, d.Name)
				}
				if e.Kind == "name" {
					out.Name = strings.Join(parts, "::")
				} else {
					out.Opcode = strings.Join(parts, "::")
				}
			}
		}
		out.Args = make([]Expr, len(e.Args))
		for j, a := range e.Args {
			out.Args[j] = rewrite(a)
		}
		if e.Body != nil {
			body := rewrite(*e.Body)
			out.Body = &body
		}
		if e.Opdef != nil {
			d := *e.Opdef
			body := rewrite(*d.Expr)
			d.Expr = &body
			out.Opdef = &d
		}
		return out
	}
	out := module
	out.Declarations = append([]Declaration{}, module.Declarations...)
	generated := []Module{}
	for j, d := range out.Declarations {
		if d.Expr != nil {
			e := rewrite(*d.Expr)
			d.Expr = &e
		}
		if d.Assumption != nil {
			e := rewrite(*d.Assumption)
			d.Assumption = &e
		}
		d.Overrides = append([]Override{}, d.Overrides...)
		for k, o := range d.Overrides {
			d.Overrides[k].Expr = rewrite(o.Expr)
		}
		if d.Kind == "instance" {
			prototype, ok := modules.Get2(d.ProtoName)
			if !ok {
				return nil, fmt.Errorf("instance module %s not found", d.ProtoName)
			}
			created, imp, err := InstantiateModule(module.Name, d, prototype, table, ids, sources, analysis)
			if err != nil {
				return nil, err
			}
			generated = append(generated, created)
			d = imp
		}
		out.Declarations[j] = d
	}
	return append(generated, out), nil
}
