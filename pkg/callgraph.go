package quint

import "github.com/glycerine/insdict"

// CallGraph maps decimal declaration IDs to the IDs they use, in discovery order.
// Edges include type aliases, lexical bindings, and import prerequisites.
type CallGraph = insdict.Dict[string, []ID]

// BuildCallGraph computes dependencies for module using a shared module context
// and a completed lookup table. It does not mutate the modules or lookup table.
func BuildCallGraph(module Module, modules []Module, table *insdict.Dict[ID, LookupDefinition]) *CallGraph {
	graph := insdict.NewDict[string, []ID]()
	hosts := insdict.NewDict[string, Module]()
	byName := insdict.NewDict[string, Module]()
	imports := insdict.NewDict[string, []ID]()
	for _, m := range modules {
		byName.Set(m.Name, m)
		for _, d := range m.Declarations {
			hosts.Set(d.ID.String(), m)
		}
	}
	for _, d := range module.Declarations {
		if d.Kind == "import" || d.Kind == "instance" {
			imports.Set(d.ProtoName, append(imports.Get(d.ProtoName), d.ID))
			if d.QualifiedName != nil {
				imports.Set(*d.QualifiedName, append(imports.Get(*d.QualifiedName), d.ID))
			}
		}
	}
	add := func(id ID, uses []ID) {
		key := id.String()
		edges := graph.Get(key)
		for _, use := range uses {
			found := false
			for _, old := range edges {
				if old == use {
					found = true
					break
				}
			}
			if !found {
				edges = append(edges, use)
			}
		}
		graph.Set(key, edges)
	}
	stack := []ID{}
	addImports := func(id ID, targets []ID) {
		host := hosts.Get(id.String())
		if host.Name != "" && host.ID != module.ID {
			for _, target := range targets {
				add(target, imports.Get(host.Name))
			}
		}
	}
	reference := func(id ID) {
		def, ok := table.Get2(id)
		if !ok {
			return
		}
		for _, caller := range stack {
			add(caller, []ID{def.ID})
		}
		addImports(def.ID, stack)
	}
	typ := func(t *Type) {
		_ = WalkType(t, func(t *Type) (bool, error) {
			if t.Kind == "const" && t.ID != nil {
				reference(*t.ID)
			}
			return true, nil
		})
	}
	var expr func(*Expr)
	expr = func(e *Expr) {
		if e == nil {
			return
		}
		if e.Kind == "let" && e.Opdef != nil {
			d := e.Opdef
			stack = append(stack, d.ID)
			addImports(d.ID, []ID{d.ID})
			typ(d.TypeAnnotation)
			expr(d.Expr)
			stack = stack[:len(stack)-1]
		}
		for _, p := range e.Params {
			typ(p.TypeAnnotation)
		}
		for i := range e.Args {
			expr(&e.Args[i])
		}
		expr(e.Body)
		if e.Kind == "name" || e.Kind == "app" {
			reference(e.ID)
		}
	}
	for _, d := range module.Declarations {
		switch d.Kind {
		case "import", "instance":
			if m := byName.Get(d.ProtoName); m.Name != "" {
				add(d.ID, []ID{m.ID})
			}
			if d.Kind == "instance" {
				stack = append(stack, d.ID)
				for i := range d.Overrides {
					expr(&d.Overrides[i].Expr)
				}
				stack = stack[:len(stack)-1]
			}
		case "export":
			add(d.ID, imports.Get(d.ProtoName))
		default:
			stack = append(stack, d.ID)
			addImports(d.ID, []ID{d.ID})
			typ(d.TypeAnnotation)
			if d.Kind == "typedef" {
				typ(d.Type)
			}
			expr(d.Expr)
			expr(d.Assumption)
			stack = stack[:len(stack)-1]
		}
	}
	return graph
}
