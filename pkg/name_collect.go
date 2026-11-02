package quint

import "github.com/glycerine/insdict"

// LocalNameCollection is the local-declaration stage of module name collection.
// Imports, instances, and exports are retained for subsequent module resolution;
// their definitions are not visible in these scopes yet.
type LocalNameCollection struct {
	Modules *insdict.Dict[string, *NameScope]
	Pending *insdict.Dict[string, []Declaration]
	Errors  []Diagnostic
}

// CollectLocalNames collects top-level declarations without traversing bodies.
// Forward references can therefore resolve after the whole module is collected.
// It leaves the input IR unchanged and follows upstream's omission of operator
// annotations from collected definitions (constant/variable annotations remain).
func CollectLocalNames(modules []Module) LocalNameCollection {
	result := LocalNameCollection{Modules: insdict.NewDict[string, *NameScope](), Pending: insdict.NewDict[string, []Declaration](), Errors: []Diagnostic{}}
	seen := map[string]bool{}
	for _, module := range modules {
		if seen[module.Name] {
			id := module.ID
			result.Errors = append(result.Errors, Diagnostic{Code: QNT102, Message: "Module with name '" + module.Name + "' was already defined", Reference: &id, Data: &DiagnosticData{}})
		}
		seen[module.Name] = true
		scope := NewNameScope(module.Name)
		pending := []Declaration{}
		for _, declaration := range module.Declarations {
			switch declaration.Kind {
			case "import", "instance", "export":
				pending = append(pending, declaration)
			case "def", "const", "var", "typedef", "assume":
				// Copy metadata before stripping annotations; never rewrite the caller's IR.
				local := declaration
				if local.Kind == "def" {
					local.TypeAnnotation = nil
				}
				depth := 0
				local.Depth = &depth
				scope.Add(LookupDefinition{ID: local.ID, Name: local.Name, Depth: 0, Declaration: &local})
			}
		}
		result.Modules.Set(module.Name, scope)
		result.Pending.Set(module.Name, pending)
		result.Errors = append(result.Errors, scope.Errors...)
	}
	return result
}
