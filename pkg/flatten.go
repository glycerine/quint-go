package quint

import "github.com/glycerine/insdict"

// FlattenModule expands exports and referenced imports while retaining instance
// directives for FlattenInstances. Modules and table must describe the same
// resolved project; neither is modified.
func FlattenModule(module Module, modules *insdict.Dict[string, Module], table *insdict.Dict[ID, LookupDefinition]) (Module, error) {
	exported, err := ExpandModuleExports(module, modules, table)
	if err != nil {
		return Module{}, err
	}
	return FlattenModuleImports(exported, table)
}
