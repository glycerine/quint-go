package quint

import (
	"github.com/glycerine/insdict"
)

// SortDeclarations orders declarations by dependency layers and then decimal ID
// strings, as upstream does. Dependencies outside declarations participate in
// ordering but are not returned. Cycles contains all residual vertices, including
// blocked dependents and nested definitions, in upstream Immutable.Set order.
func SortDeclarations(graph *CallGraph, declarations []Declaration) (sorted []Declaration, cycles []ID) {
	byID := insdict.NewDict[string, Declaration]()
	ids := make([]ID, len(declarations))
	for i, d := range declarations {
		byID.Set(d.ID.String(), d)
		ids[i] = d.ID
	}
	ordered, cycles := sortDependencyIDs(graph, ids)
	sorted = []Declaration{}
	for _, id := range ordered {
		sorted = append(sorted, byID.Get(id.String()))
	}
	return sorted, cycles
}

// SortModuleDeclarations applies the parser's declaration-ordering phase to
// resolved modules, retaining sortable declarations even when cycles exist.
func SortModuleDeclarations(modules []Module, table *insdict.Dict[ID, LookupDefinition]) ([]Module, []Diagnostic) {
	result := append([]Module{}, modules...)
	diagnostics := []Diagnostic{}
	for i, m := range modules {
		sorted, cycles := SortDeclarations(BuildCallGraph(m, modules, table), m.Declarations)
		result[i].Declarations = sorted
		for _, id := range cycles {
			diagnostics = append(diagnostics, Diagnostic{Code: QNT099, Message: "Found cyclic declarations. Use fold and foldl instead of recursion", Reference: &id})
		}
	}
	return result, diagnostics
}
