package quint

// runtimeReferenceOrder returns Immutable.Map constructor key traversal,
// including mutable constructor ownership, equal-key source retention, and
// ninth-key HAMT conversion. It deliberately uses the same node machinery as
// retained runtime collections. Deletions/updates must use the persistent map
// rather than reconstructing from final values.
func runtimeReferenceOrder(values []RuntimeValue) ([]RuntimeValue, error) {
	entries := make([]runtimeReferenceEntry, len(values))
	for i, value := range values {
		entries[i] = runtimeReferenceEntry{Key: value, Value: value}
	}
	m, err := runtimeReferenceMapFromEntries(entries)
	if err != nil {
		return nil, err
	}
	ordered := make([]RuntimeValue, 0, m.Len())
	for _, entry := range m.Entries() {
		ordered = append(ordered, entry.Key)
	}
	return ordered, nil
}
