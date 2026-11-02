package quint

import "fmt"

// ResolveLocalModule resolves a module without import/export/instance directives.
// Module linking must supply imported visibility before this restriction can be
// lifted. It returns ordinary collection and reference diagnostics together.
func ResolveLocalModule(module Module) (*ExpressionNameResolver, error) {
	for _, d := range module.Declarations {
		if d.Kind == "import" || d.Kind == "export" || d.Kind == "instance" {
			return nil, fmt.Errorf("module %s requires module linking for %s declarations", module.Name, d.Kind)
		}
	}
	collection := CollectLocalNames([]Module{module})
	resolver := NewExpressionNameResolver(collection.Modules.Get(module.Name))
	resolver.Errors = append(resolver.Errors, collection.Errors...)
	for i := range module.Declarations {
		d := module.Declarations[i]
		resolver.resolveType(d.TypeAnnotation)
		if d.Kind == "typedef" {
			resolver.resolveType(d.Type)
		}
		switch d.Kind {
		case "def":
			resolver.Table.Set(d.ID, LookupDefinition{ID: d.ID, Name: d.Name, Depth: 0, Declaration: &d})
			resolver.Resolve(d.Expr, 0)
		case "assume":
			resolver.Resolve(d.Assumption, 0)
		}
	}
	return resolver, nil
}
