package quint

import (
	"sort"
	"strings"
)

// SortModules orders dependencies before importers. As in upstream's Kahn sort,
// each layer is ordered by the decimal ID string (JavaScript's default sort).
// On error the original order is returned. Residual cycle diagnostics include
// modules blocked by a cycle, not just members of strongly connected components.
func SortModules(modules []Module) ([]Module, []Diagnostic) {
	byName := map[string]int{}
	diagnostics := []Diagnostic{}
	for i, m := range modules {
		if _, ok := byName[m.Name]; ok {
			id := m.ID
			diagnostics = append(diagnostics, Diagnostic{Code: QNT101, Message: "Multiple modules conflict on the same name: " + m.Name, Reference: &id})
		}
		byName[m.Name] = i
	}
	if len(diagnostics) > 0 {
		return modules, diagnostics
	}
	deps := make([]map[int]bool, len(modules))
	for i, m := range modules {
		deps[i] = map[int]bool{}
		for _, d := range m.Declarations {
			if d.Kind != "import" && d.Kind != "instance" {
				continue
			}
			j, ok := byName[d.ProtoName]
			if !ok {
				id := d.ID
				return modules, []Diagnostic{{Code: QNT405, Message: "Module " + m.Name + " imports an unknown module " + d.ProtoName, Reference: &id}}
			}
			deps[i][j] = true
		}
	}
	done := make([]bool, len(modules))
	sorted := make([]Module, 0, len(modules))
	for {
		layer := []int{}
		for i := range modules {
			if !done[i] && len(deps[i]) == 0 {
				layer = append(layer, i)
			}
		}
		if len(layer) == 0 {
			break
		}
		sort.Slice(layer, func(i, j int) bool { return modules[layer[i]].ID.String() < modules[layer[j]].ID.String() })
		for _, i := range layer {
			done[i] = true
			sorted = append(sorted, modules[i])
		}
		for _, uses := range deps {
			for _, i := range layer {
				delete(uses, i)
			}
		}
	}
	if len(sorted) == len(modules) {
		return sorted, diagnostics
	}
	names := []string{}
	var reference *ID
	for i, m := range modules {
		if !done[i] {
			names = append(names, m.Name)
			if reference == nil {
				id := m.ID
				reference = &id
			}
		}
	}
	return modules, []Diagnostic{{Code: QNT098, Message: "Cyclic imports among: " + strings.Join(names, ", "), Reference: reference}}
}

// LoadAndSortModules loads file imports then checks and orders module dependencies.
func LoadAndSortModules(resolver SourceResolver, main SourceLookupPath) ([]Module, error) {
	modules, err := LoadSourceModules(resolver, main)
	if err != nil {
		return nil, err
	}
	sorted, diagnostics := SortModules(modules)
	if len(diagnostics) > 0 {
		return nil, diagnostics[0]
	}
	return sorted, nil
}
