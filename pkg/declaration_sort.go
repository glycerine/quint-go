package quint

import (
	"github.com/glycerine/insdict"
	"sort"
)

// SortDeclarations orders declarations by dependency layers and then decimal ID
// strings, as upstream does. Dependencies outside declarations participate in
// ordering but are not returned. Cycles contains all residual vertices, including
// blocked dependents and nested definitions, in graph discovery order.
func SortDeclarations(graph *CallGraph, declarations []Declaration) (sorted []Declaration, cycles []ID) {
	edges := insdict.NewDict[string, []ID]()
	for id, uses := range graph.All() {
		edges.Set(id, append([]ID(nil), uses...))
	}
	byID := insdict.NewDict[string, Declaration]()
	for _, d := range declarations {
		byID.Set(d.ID.String(), d)
	}
	// Complete the vertex set before removing any dependency layers.
	for _, uses := range graph.All() {
		for _, id := range uses {
			if _, ok := edges.Get2(id.String()); !ok {
				edges.Set(id.String(), nil)
			}
		}
	}
	for _, d := range declarations {
		if _, ok := edges.Get2(d.ID.String()); !ok {
			edges.Set(d.ID.String(), nil)
		}
	}
	done := map[string]bool{}
	sorted = []Declaration{}
	cycles = []ID{}
	for {
		sinks := []string{}
		for id, uses := range edges.All() {
			if !done[id] && len(uses) == 0 {
				sinks = append(sinks, id)
			}
		}
		if len(sinks) == 0 {
			break
		}
		sort.Strings(sinks)
		for _, id := range sinks {
			done[id] = true
			if d, ok := byID.Get2(id); ok {
				sorted = append(sorted, d)
			}
		}
		for id, uses := range edges.All() {
			if done[id] {
				continue
			}
			remaining := []ID{}
			for _, use := range uses {
				if !done[use.String()] {
					remaining = append(remaining, use)
				}
			}
			edges.Set(id, remaining)
		}
	}
	for key := range edges.All() {
		if !done[key] {
			id, _ := ParseID(key)
			cycles = append(cycles, id)
		}
	}
	return
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
