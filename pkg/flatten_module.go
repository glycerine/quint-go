package quint

import (
	"fmt"
	"github.com/glycerine/insdict"
)

// FlattenModuleImports expands referenced imports into dependency-ordered
// definitions. Export expansion and instance instantiation are separate stages;
// unresolved exports are rejected rather than silently discarded.
func FlattenModuleImports(module Module, table *insdict.Dict[ID, LookupDefinition]) (Module, error) {
	out := module
	out.Declarations = []Declaration{}
	appendDeclaration := func(d Declaration) {
		for _, old := range out.Declarations {
			if d.Kind == "instance" {
				if old.Kind == "instance" && old.ID == d.ID {
					return
				}
			} else if old.Kind != "instance" && old.Name == d.Name {
				return
			}
		}
		out.Declarations = append(out.Declarations, d)
	}
	collect := func(e Expr) error {
		ds, err := DependentDefinitions(e, table)
		if err != nil {
			return err
		}
		for _, d := range ds {
			appendDeclaration(d)
		}
		return nil
	}
	for _, d := range module.Declarations {
		if d.Kind == "export" {
			return Module{}, fmt.Errorf("export expansion is required before FlattenModuleImports")
		}
		if d.Kind == "import" {
			continue
		}
		if d.Expr != nil {
			if err := collect(*d.Expr); err != nil {
				return Module{}, err
			}
		}
		if d.Assumption != nil {
			if err := collect(*d.Assumption); err != nil {
				return Module{}, err
			}
		}
		for _, o := range d.Overrides {
			if err := collect(o.Expr); err != nil {
				return Module{}, err
			}
		}
		appendDeclaration(d)
	}
	return out, nil
}
