package quint

import (
	"fmt"
	"math/bits"
)

// runtimeReferenceMapFromEntries follows Immutable.Map(entries), which inserts
// all pairs in one withMutations owner batch. Keys must already have the normal
// forms required by the caller. The result and input entry slice are independent.
func runtimeReferenceMapFromEntries(entries []runtimeReferenceEntry) (runtimeReferenceMap, error) {
	return runtimeReferenceMapBatchSet(runtimeReferenceMap{}, entries)
}

// runtimeReferenceMapBatchSet inserts entries in a new withMutations owner batch.
// It preserves base and shares untouched immutable nodes. An editable ValueNode
// retains its existing key object when an equal key replaces its value; the
// first replacement of a borrowed leaf copies it with the incoming key instead.
// Array and collision nodes replace the key object on all non-noop updates.
func runtimeReferenceMapBatchSet(base runtimeReferenceMap, entries []runtimeReferenceEntry) (runtimeReferenceMap, error) {
	builder := referenceMapBuilder{owned: make(map[*runtimeReferenceMapNode]bool)}
	result := base
	for _, entry := range entries {
		if !entry.Value.Valid() {
			return runtimeReferenceMap{}, fmt.Errorf("reference map requires a valid runtime value")
		}
		hash, err := runtimeReferenceHash(entry.Key)
		if err != nil {
			return runtimeReferenceMap{}, err
		}
		if result.root == nil {
			result.root = builder.own(&runtimeReferenceMapNode{kind: referenceArrayNode, entries: []runtimeReferenceEntry{entry}})
			result.size = 1
			continue
		}
		var delta int
		result.root, delta, err = builder.insert(result.root, 0, uint32(hash), entry)
		if err != nil {
			return runtimeReferenceMap{}, err
		}
		result.size += delta
	}
	return result, nil
}

// The ownership lookup is private to one batch; it is discarded before any
// resulting map is published. Only nodes in this set may be mutated in place.
type referenceMapBuilder struct {
	owned map[*runtimeReferenceMapNode]bool
}

func (b *referenceMapBuilder) own(node *runtimeReferenceMapNode) *runtimeReferenceMapNode {
	b.owned[node] = true
	return node
}
func (b *referenceMapBuilder) editable(node *runtimeReferenceMapNode) *runtimeReferenceMapNode {
	if b.owned[node] {
		return node
	}
	copy := *node
	copy.entries = append([]runtimeReferenceEntry(nil), node.entries...)
	copy.nodes = append([]*runtimeReferenceMapNode(nil), node.nodes...)
	return b.own(&copy)
}
func (b *referenceMapBuilder) leaf(hash uint32, entry runtimeReferenceEntry) *runtimeReferenceMapNode {
	return b.own(referenceValue(hash, entry))
}
func (b *referenceMapBuilder) merge(node *runtimeReferenceMapNode, shift uint, hash uint32, entry runtimeReferenceEntry) *runtimeReferenceMapNode {
	if node.hash == hash {
		return b.own(&runtimeReferenceMapNode{kind: referenceCollisionNode, hash: hash, entries: []runtimeReferenceEntry{node.entries[0], entry}})
	}
	first, second := (node.hash>>shift)&31, (hash>>shift)&31
	bitmap := (uint32(1) << first) | (uint32(1) << second)
	var children []*runtimeReferenceMapNode
	if first == second {
		children = []*runtimeReferenceMapNode{b.merge(node, shift+5, hash, entry)}
	} else if first < second {
		children = []*runtimeReferenceMapNode{node, b.leaf(hash, entry)}
	} else {
		children = []*runtimeReferenceMapNode{b.leaf(hash, entry), node}
	}
	return b.own(&runtimeReferenceMapNode{kind: referenceBitmapNode, bitmap: bitmap, nodes: children})
}
func (b *referenceMapBuilder) insert(node *runtimeReferenceMapNode, shift uint, hash uint32, entry runtimeReferenceEntry) (*runtimeReferenceMapNode, int, error) {
	if node == nil {
		return b.leaf(hash, entry), 1, nil
	}
	switch node.kind {
	case referenceArrayNode, referenceCollisionNode:
		if node.kind == referenceCollisionNode && hash != node.hash {
			return b.merge(node, shift, hash, entry), 1, nil
		}
		for index, existing := range node.entries {
			if existing.Key.Equal(entry.Key) {
				if sameRuntimeObject(existing.Value, entry.Value) {
					return node, 0, nil
				}
				copy := b.editable(node)
				copy.entries[index] = entry
				return copy, 0, nil
			}
		}
		if node.kind == referenceArrayNode && len(node.entries) >= 8 {
			// The ninth key becomes the first insertion into the new trie.
			root := b.leaf(hash, entry)
			for _, existing := range node.entries {
				existingHash, err := runtimeReferenceHash(existing.Key)
				if err != nil {
					return nil, 0, err
				}
				root, _, err = b.insert(root, 0, uint32(existingHash), existing)
				if err != nil {
					return nil, 0, err
				}
			}
			return root, 1, nil
		}
		copy := b.editable(node)
		copy.entries = append(copy.entries, entry)
		return copy, 1, nil
	case referenceValueNode:
		if node.entries[0].Key.Equal(entry.Key) {
			if sameRuntimeObject(node.entries[0].Value, entry.Value) {
				return node, 0, nil
			}
			wasEditable := b.owned[node]
			copy := b.editable(node)
			if wasEditable {
				copy.entries[0].Value = entry.Value
			} else {
				copy.entries[0] = entry
			}
			return copy, 0, nil
		}
		return b.merge(node, shift, hash, entry), 1, nil
	case referenceBitmapNode:
		fragment := (hash >> shift) & 31
		bit := uint32(1) << fragment
		exists := node.bitmap&bit != 0
		index := bits.OnesCount32(node.bitmap & (bit - 1))
		var old *runtimeReferenceMapNode
		if exists {
			old = node.nodes[index]
		}
		child, delta, err := b.insert(old, shift+5, hash, entry)
		if err != nil || child == old {
			return node, delta, err
		}
		if !exists && len(node.nodes) >= 16 {
			dense := make([]*runtimeReferenceMapNode, 32)
			oldIndex := 0
			for slot := 0; slot < 32; slot++ {
				if node.bitmap&(uint32(1)<<uint(slot)) != 0 {
					dense[slot] = node.nodes[oldIndex]
					oldIndex++
				}
			}
			dense[fragment] = child
			return b.own(&runtimeReferenceMapNode{kind: referenceDenseNode, count: len(node.nodes) + 1, nodes: dense}), delta, nil
		}
		copyNode := b.editable(node)
		if exists {
			copyNode.nodes[index] = child
		} else {
			copyNode.bitmap |= bit
			copyNode.nodes = append(copyNode.nodes, nil)
			copy(copyNode.nodes[index+1:], copyNode.nodes[index:])
			copyNode.nodes[index] = child
		}
		return copyNode, delta, nil
	case referenceDenseNode:
		index := (hash >> shift) & 31
		old := node.nodes[index]
		child, delta, err := b.insert(old, shift+5, hash, entry)
		if err != nil || child == old {
			return node, delta, err
		}
		copy := b.editable(node)
		if old == nil {
			copy.count++
		}
		copy.nodes[index] = child
		return copy, delta, nil
	default:
		return nil, 0, fmt.Errorf("invalid reference map node")
	}
}
