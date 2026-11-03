package quint

import (
	"sort"

	"github.com/glycerine/insdict"
)

// Immutable hashes BigInt keys through their decimal strings. These private
// ordering views share the tested reference HAMT with runtime collections;
// logical dependency data remains in insertion-ordered dictionaries.
type dependencyKeyOrder struct {
	keys *insdict.Dict[string, RuntimeValue]
}

func newDependencyKeyOrder() *dependencyKeyOrder {
	return &dependencyKeyOrder{keys: insdict.NewDict[string, RuntimeValue]()}
}

func (o *dependencyKeyOrder) key(id string) RuntimeValue {
	if value, ok := o.keys.Get2(id); ok {
		return value
	}
	value, _ := NewRuntimeValue(Expr{Kind: "str", Str: id})
	o.keys.Set(id, value)
	return value
}

func (o *dependencyKeyOrder) add(order runtimeReferenceMap, id string) runtimeReferenceMap {
	value := o.key(id)
	order, _ = order.Set(value, value)
	return order
}

func (o *dependencyKeyOrder) construct(ids []string) runtimeReferenceMap {
	entries := make([]runtimeReferenceEntry, len(ids))
	for i, id := range ids {
		value := o.key(id)
		entries[i] = runtimeReferenceEntry{Key: value, Value: value}
	}
	order, _ := runtimeReferenceMapFromEntries(entries)
	return order
}

func dependencyUnion(base, additional runtimeReferenceMap) runtimeReferenceMap {
	if additional.Len() == 0 {
		return base
	}
	if base.Len() == 0 {
		return additional
	}
	order, _ := runtimeReferenceMapBatchSet(base, additional.Entries())
	return order
}

func dependencyOrderKeys(order runtimeReferenceMap) []string {
	keys := make([]string, 0, order.Len())
	for entry := range order.all() {
		keys = append(keys, entry.Key.expression.Str)
	}
	return keys
}

// sortDependencyIDs follows upstream's collection operations as well as Kahn's
// layers. In particular partition rebuilds its maps and the final cycle Set
// rebuilds its key order; filtering the original discovery order differs above
// eight vertices and for colliding arbitrary-precision IDs.
func sortDependencyIDs(graph *CallGraph, unsorted []ID) (sorted, cycles []ID) {
	ordering := newDependencyKeyOrder()
	edges := insdict.NewDict[string, []ID]()
	var order runtimeReferenceMap
	for key, uses := range graph.All() {
		edges.Set(key, append([]ID(nil), uses...))
		order = ordering.add(order, key)
	}
	unsortedKeys := make([]string, len(unsorted))
	present := map[string]bool{}
	for i, id := range unsorted {
		unsortedKeys[i] = id.String()
		present[id.String()] = true
	}
	allIDs := ordering.construct(unsortedKeys)
	for _, key := range dependencyOrderKeys(order) {
		var usesOrder runtimeReferenceMap
		for _, id := range edges.Get(key) {
			usesOrder = ordering.add(usesOrder, id.String())
		}
		allIDs = dependencyUnion(usesOrder, allIDs)
	}
	for _, key := range dependencyOrderKeys(allIDs) {
		if _, ok := edges.Get2(key); !ok {
			edges.Set(key, nil)
			order = ordering.add(order, key)
		}
	}
	sorted, cycles = []ID{}, []ID{}
	for {
		sinks, others := []string{}, []string{}
		for _, key := range dependencyOrderKeys(order) {
			if len(edges.Get(key)) == 0 {
				sinks = append(sinks, key)
			} else {
				others = append(others, key)
			}
		}
		order = ordering.construct(others)
		if len(sinks) == 0 {
			break
		}
		sort.Strings(sinks)
		removed := map[string]bool{}
		for _, key := range sinks {
			removed[key] = true
			if present[key] {
				id, _ := ParseID(key)
				sorted = append(sorted, id)
			}
		}
		for _, key := range others {
			remaining := []ID{}
			for _, id := range edges.Get(key) {
				if !removed[id.String()] {
					remaining = append(remaining, id)
				}
			}
			edges.Set(key, remaining)
		}
	}
	cycleSet := ordering.construct(dependencyOrderKeys(order))
	for _, key := range dependencyOrderKeys(cycleSet) {
		id, _ := ParseID(key)
		cycles = append(cycles, id)
	}
	return sorted, cycles
}

func dependencyNameSetOrder(names []string) []string {
	return dependencyOrderKeys(newDependencyKeyOrder().construct(names))
}
