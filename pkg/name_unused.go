package quint

func cloneLookupEntries(entries []LookupDefinition) []LookupDefinition {
	result := append([]LookupDefinition(nil), entries...)
	for i := range result {
		result[i].identity = new(byte)
	}
	return result
}

// UnusedDefinitions returns scope entries that no lookup reference uses.
// Identity is per collected binding, not per IR ID: an imported copy can be
// unused even when its prototype is used. Definition self-entries do not count
// as references. The result follows scope and definition-stack insertion order.
func (r LinkedNameResolution) UnusedDefinitions(module string) []LookupDefinition {
	result := []LookupDefinition{}
	scope := r.Modules.Get(module)
	if scope == nil {
		return result
	}
	used := map[*byte]bool{}
	for _, def := range r.Table.All() {
		if def.identity != nil {
			used[def.identity] = true
		}
	}
	seen := map[*byte]bool{}
	for _, entries := range scope.Definitions.All() {
		for _, def := range entries {
			if !used[def.identity] && !seen[def.identity] {
				result = append(result, def)
				seen[def.identity] = true
			}
		}
	}
	return result
}
