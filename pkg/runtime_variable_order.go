package quint

// RuntimeVariableBinding links a flattened variable name to its source runtime
// cell. Key includes the source declaration ID and instance namespace; Name is
// the displayed state field, which ordinary import aliases do not change.
type RuntimeVariableBinding struct {
	Key, Name string
}

// VarNames returns registered variable names in the reference runtime's map
// traversal order. Cells remain in the insertion-ordered storage dictionary;
// this view models Immutable.Map's constructor-independent persistent inserts,
// including its conversion to a hash trie after the eighth distinct key.
// Reset, shifts, and recovery retain the registered keys and their history.
func (s *VariableStorage) VarNames() []string {
	if s == nil || s.current == nil {
		return []string{}
	}
	var order runtimeReferenceMap
	for key, register := range s.current.All() {
		keyValue, _ := NewRuntimeValue(Expr{Kind: "str", Str: key})
		nameValue, _ := NewRuntimeValue(Expr{Kind: "str", Str: register.Name})
		order, _ = order.Set(keyValue, nameValue)
	}
	names := make([]string, 0, order.Len())
	for _, entry := range order.Entries() {
		names = append(names, entry.Value.expression.Str)
	}
	return names
}
