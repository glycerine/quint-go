package quint

import (
	"strings"

	"github.com/glycerine/insdict"
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
	graph := insdict.NewDict[string, []ID]()
	ids := make([]ID, len(modules))
	byID := insdict.NewDict[string, Module]()
	for i, m := range modules {
		ids[i] = m.ID
		byID.Set(m.ID.String(), m)
		imports := []ID{}
		for _, d := range m.Declarations {
			if d.Kind != "import" && d.Kind != "instance" {
				continue
			}
			j, ok := byName[d.ProtoName]
			if !ok {
				id := d.ID
				return modules, []Diagnostic{{Code: QNT405, Message: "Module " + m.Name + " imports an unknown module " + d.ProtoName, Reference: &id}}
			}
			imports = append(imports, modules[j].ID)
		}
		// Upstream inserts an edge-map key only while visiting declarations.
		// Empty modules are added later by toposort's vertex completion.
		if len(m.Declarations) > 0 {
			graph.Set(m.ID.String(), imports)
		}
	}
	ordered, cycles := sortDependencyIDs(graph, ids)
	if len(cycles) == 0 {
		sorted := make([]Module, 0, len(ordered))
		for _, id := range ordered {
			sorted = append(sorted, byID.Get(id.String()))
		}
		return sorted, diagnostics
	}
	names := []string{}
	for _, id := range cycles {
		names = append(names, byID.Get(id.String()).Name)
	}
	// Set.map rebuilds under the names' hashes. Its message order is independent
	// of cycles.first(), which still selects the reference from the BigInt set.
	names = dependencyNameSetOrder(names)
	reference := cycles[0]
	return modules, []Diagnostic{{Code: QNT098, Message: "Cyclic imports among: " + strings.Join(names, ", "), Reference: &reference}}
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
