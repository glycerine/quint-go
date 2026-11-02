package quint

import "github.com/glycerine/insdict"

// FlattenModule expands exports and referenced imports while retaining instance
// directives for FlattenInstances. Modules and table must describe the same
// resolved project; neither is modified.
func FlattenModule(module Module, modules *insdict.Dict[string, Module], table *insdict.Dict[ID, LookupDefinition]) (Module, error) {
	registry := insdict.NewDict[string, Module]()
	for name, m := range modules.All() {
		registry.Set(name, m)
	}
	out := module
	out.Declarations = []Declaration{}
	for _, d := range module.Declarations {
		if (d.Kind == "import" || d.Kind == "instance") && d.QualifiedName != nil {
			if m, ok := registry.Get2(d.ProtoName); ok {
				registry.Set(*d.QualifiedName, m)
			}
		}
		part := module
		part.Declarations = []Declaration{d}
		var expanded Module
		var err error
		if d.Kind == "export" {
			expanded, err = ExpandModuleExports(part, registry, table)
		} else {
			expanded, err = FlattenModuleImports(part, table)
		}
		if err != nil {
			return Module{}, err
		}
		for _, candidate := range expanded.Declarations {
			found := false
			for _, old := range out.Declarations {
				if candidate.Kind == "instance" {
					found = old.Kind == "instance" && old.ID == candidate.ID
				} else {
					found = old.Kind != "instance" && old.Name == candidate.Name
				}
				if found {
					break
				}
			}
			if !found {
				out.Declarations = append(out.Declarations, candidate)
			}
		}
	}
	return out, nil
}
