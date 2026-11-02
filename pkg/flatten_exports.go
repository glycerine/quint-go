package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
	"strings"
)

// ExpandModuleExports replaces exports with referenced top-level definitions.
// Modules must be supplied in lookup order; aliases are recorded as encountered.
func ExpandModuleExports(module Module, modules *insdict.Dict[string, Module], table *insdict.Dict[ID, LookupDefinition]) (Module, error) {
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
		if d.Kind != "export" {
			out.Declarations = append(out.Declarations, d)
			continue
		}
		target, ok := registry.Get2(d.ProtoName)
		if !ok {
			return Module{}, fmt.Errorf("export module %s not found", d.ProtoName)
		}
		ids := map[ID]bool{}
		for _, decl := range target.Declarations {
			ids[decl.ID] = true
		}
		namespace := d.ProtoName
		if d.QualifiedName != nil {
			namespace = *d.QualifiedName
		}
		if d.DefName != nil {
			namespace = ""
		}
		for _, def := range table.All() {
			if !ids[def.ID] || def.Declaration == nil {
				continue
			}
			copied := *def.Declaration
			depth := def.Depth
			copied.Depth = &depth
			copied.Name = def.Name
			if namespace != "" && !strings.HasPrefix(copied.Name, namespace+"::") {
				var err error
				copied, err = AddNamespaceToDefinition(copied, namespace, builtinResolutionNames)
				if err != nil {
					return Module{}, err
				}
			}
			exists := false
			for _, old := range out.Declarations {
				if old.Name == copied.Name {
					exists = true
					break
				}
			}
			if exists {
				continue
			}
			// Discover dependencies with the namespace applied to references.
			if copied.Expr != nil {
				dependencies, err := dependentDefinitionsInNamespace(*copied.Expr, table, namespace)
				if err != nil {
					return Module{}, err
				}
				out.Declarations = append(out.Declarations, dependencies...)
			}
			hidden := false
			copied.Hidden = &hidden
			copied.Namespaces = []string{}
			copied.ImportedFrom = nil
			out.Declarations = append(out.Declarations, copied)
		}
	}
	return out, nil
}
