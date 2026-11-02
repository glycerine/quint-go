package quint

import (
	"encoding/json"
	"fmt"
	"github.com/glycerine/insdict"
	"strings"
)

// DependentDefinitions collects referenced definitions in dependency order for
// flattening. Instance imports are handled by the separate instantiation pass.
func DependentDefinitions(expression Expr, table *insdict.Dict[ID, LookupDefinition]) ([]Declaration, error) {
	return dependentDefinitionsInNamespace(expression, table, "")
}
func dependentDefinitionsInNamespace(expression Expr, table *insdict.Dict[ID, LookupDefinition], namespace string) ([]Declaration, error) {
	out := []Declaration{}
	active := map[string]bool{}
	var visit func(Expr, string) error
	var reference func(ID, string) error
	reference = func(id ID, inherited string) error {
		def, ok := table.Get2(id)
		if !ok || def.Parameter != nil || def.Declaration == nil {
			return nil
		}
		d := *def.Declaration
		if def.ImportedFrom != nil {
			hidden := def.Hidden
			d.Hidden = &hidden
			d.Namespaces = append([]string{}, def.Namespaces...)
			d.ImportedFrom, _ = json.Marshal(def.ImportedFrom)
		}
		if d.Kind == "def" && def.Depth > 0 {
			return nil
		}
		if def.ImportedFrom != nil && def.ImportedFrom.Kind == "instance" {
			return nil
		}
		namespace := inherited
		if namespace == "" {
			parts := append([]string{}, def.Namespaces...)
			for l, r := 0, len(parts)-1; l < r; l, r = l+1, r-1 {
				parts[l], parts[r] = parts[r], parts[l]
			}
			namespace = strings.Join(parts, "::")
		}
		d.Name = def.Name
		if namespace != "" && !strings.HasPrefix(d.Name, namespace+"::") {
			var err error
			d, err = AddNamespaceToDefinition(d, namespace, builtinResolutionNames)
			if err != nil {
				return err
			}
		}
		if active[d.Name] {
			return fmt.Errorf("cyclic flattening dependency %s", d.Name)
		}
		active[d.Name] = true
		if d.Expr != nil {
			if err := visit(*d.Expr, namespace); err != nil {
				return err
			}
		}
		if d.Assumption != nil {
			if err := visit(*d.Assumption, namespace); err != nil {
				return err
			}
		}
		delete(active, d.Name)
		for _, previous := range out {
			if previous.Name == d.Name {
				return nil
			}
		}
		out = append(out, d)
		return nil
	}
	visit = func(e Expr, namespace string) error {
		if e.Kind == "name" || e.Kind == "app" {
			if err := reference(e.ID, namespace); err != nil {
				return err
			}
		}
		if e.Opdef != nil {
			if err := visit(*e.Opdef.Expr, namespace); err != nil {
				return err
			}
		}
		for _, a := range e.Args {
			if err := visit(a, namespace); err != nil {
				return err
			}
		}
		if e.Body != nil {
			return visit(*e.Body, namespace)
		}
		return nil
	}
	err := visit(expression, namespace)
	return out, err
}
