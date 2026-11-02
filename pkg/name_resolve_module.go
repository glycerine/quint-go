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
	resolver.resolveModuleBody(module)
	return resolver, nil
}
func (resolver *ExpressionNameResolver) resolveModuleBody(module Module) {
	for i := range module.Declarations {
		d := module.Declarations[i]
		resolver.resolveType(d.TypeAnnotation)
		if d.Kind == "typedef" {
			resolver.resolveType(d.Type)
		}
		switch d.Kind {
		case "def":
			definition := declarationLookup(d)
			definition.Depth = 0
			resolver.Table.Set(d.ID, definition)
			resolver.Resolve(d.Expr, 0)
		case "instance":
			for _, override := range d.Overrides {
				name := override.Parameter.Name
				if d.QualifiedName != nil {
					name = *d.QualifiedName + "::" + name
				}
				resolver.resolve(name, override.Parameter.ID, false)
			}
			for _, override := range d.Overrides {
				resolver.Resolve(&override.Expr, 0)
			}
		case "assume":
			resolver.Resolve(d.Assumption, 0)
		}
	}
}
