package quint

import (
	"github.com/glycerine/insdict"
)

// LinkedNameResolution keeps the lookup table and visible scopes after linking.
type LinkedNameResolution struct {
	Table   *insdict.Dict[ID, LookupDefinition]
	Modules *insdict.Dict[string, *NameScope]
	Errors  []Diagnostic
}

// ResolveLinkedModules expects dependency-ordered modules (see SortModules).
// Imports and exports are processed in declaration order, then all references
// are resolved against the completed scope, including instance overrides.
func ResolveLinkedModules(modules []Module) (LinkedNameResolution, error) {
	resolver := NewExpressionNameResolver(nil)
	result := LinkedNameResolution{Table: resolver.Table, Modules: insdict.NewDict[string, *NameScope](), Errors: []Diagnostic{}}
	for _, m := range modules {
		if result.Modules.Get(m.Name) != nil {
			id := m.ID
			result.Errors = append(result.Errors, Diagnostic{Code: QNT102, Message: "Module with name '" + m.Name + "' was already defined", Reference: &id, Data: &DiagnosticData{}})
		}
		scope := NewNameScope(m.Name)
		for _, d := range m.Declarations {
			switch d.Kind {
			case "import":
				source := result.Modules.Get(d.ProtoName)
				if source != nil && d.ProtoName != m.Name && d.QualifiedName != nil {
					alias := NewNameScope(*d.QualifiedName)
					for name, entries := range source.Definitions.All() {
						alias.Definitions.Set(name, cloneLookupEntries(entries))
					}
					result.Modules.Set(*d.QualifiedName, alias)
				}
				scope.ImportDefinitions(source, d)
			case "instance":
				instance := scope.InstanceDefinitions(result.Modules.Get(d.ProtoName), d)
				if instance != nil && d.QualifiedName != nil {
					result.Modules.Set(*d.QualifiedName, instance)
				}
			case "export":
				scope.ExportDefinitions(result.Modules.Get(d.ProtoName), d)
			case "def", "const", "var", "typedef", "assume":
				local := d
				if local.Kind == "def" {
					local.TypeAnnotation = nil
				}
				depth := 0
				local.Depth = &depth
				scope.Add(LookupDefinition{ID: local.ID, Name: local.Name, Declaration: &local})
			}
		}
		result.Modules.Set(m.Name, scope)
		result.Errors = append(result.Errors, scope.Errors...)
		resolver.Scope = scope
		resolver.Errors = nil
		resolver.resolveModuleBody(m)
		result.Errors = append(result.Errors, resolver.Errors...)
	}
	return result, nil
}
