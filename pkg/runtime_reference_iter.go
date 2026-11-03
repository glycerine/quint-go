package quint

import "iter"

// all visits immutable entries in the same node order as Entries, without
// allocating an intermediate slice. Entries remains the owned slice boundary.
func (m runtimeReferenceMap) all() iter.Seq[runtimeReferenceEntry] {
	return func(yield func(runtimeReferenceEntry) bool) {
		walkRuntimeReferenceEntries(m.root, yield)
	}
}

func walkRuntimeReferenceEntries(node *runtimeReferenceMapNode, yield func(runtimeReferenceEntry) bool) bool {
	if node == nil {
		return true
	}
	switch node.kind {
	case referenceArrayNode, referenceCollisionNode, referenceValueNode:
		for _, entry := range node.entries {
			if !yield(entry) {
				return false
			}
		}
	default:
		for _, child := range node.nodes {
			if !walkRuntimeReferenceEntries(child, yield) {
				return false
			}
		}
	}
	return true
}
