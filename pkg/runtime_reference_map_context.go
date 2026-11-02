package quint

import (
	"context"
	"fmt"
	"math/bits"
)

// Contextual HAMT operations preserve the reference's raw key hashes and node
// history. Key equality can enumerate compact payloads and returns its errors;
// it is always evaluated with the incoming key first, matching Immutable.is.
// Equal keys with different raw hashes may coexist in separate trie branches.
// These helpers deliberately preserve that upstream behavior rather than
// searching other branches or substituting the public extensional HashContext.
func (m runtimeReferenceMap) LookupContext(ctx context.Context, key RuntimeValue) (RuntimeValue, bool, error) {
	hash, err := runtimeReferenceHashContext(ctx, key)
	if err != nil {
		return RuntimeValue{}, false, err
	}
	return m.root.lookupContext(ctx, 0, uint32(hash), key)
}
func (m runtimeReferenceMap) SetContext(ctx context.Context, key, value RuntimeValue) (runtimeReferenceMap, error) {
	if err := ctx.Err(); err != nil {
		return m, err
	}
	if !value.Valid() {
		return m, fmt.Errorf("reference map requires a valid runtime value")
	}
	hash, err := runtimeReferenceHashContext(ctx, key)
	if err != nil {
		return m, err
	}
	entry := runtimeReferenceEntry{key, value}
	if m.root == nil {
		return runtimeReferenceMap{root: &runtimeReferenceMapNode{kind: referenceArrayNode, entries: []runtimeReferenceEntry{entry}}, size: 1}, nil
	}
	root, delta, err := m.root.updateContext(ctx, 0, uint32(hash), entry, false)
	if err != nil {
		return m, err
	}
	return runtimeReferenceMap{root: root, size: m.size + delta}, nil
}
func (m runtimeReferenceMap) DeleteContext(ctx context.Context, key RuntimeValue) (runtimeReferenceMap, error) {
	hash, err := runtimeReferenceHashContext(ctx, key)
	if err != nil {
		return m, err
	}
	if m.root == nil {
		return m, nil
	}
	root, delta, err := m.root.updateContext(ctx, 0, uint32(hash), runtimeReferenceEntry{Key: key}, true)
	if err != nil {
		return m, err
	}
	return runtimeReferenceMap{root: root, size: m.size + delta}, nil
}
func (n *runtimeReferenceMapNode) lookupContext(ctx context.Context, shift uint, hash uint32, key RuntimeValue) (RuntimeValue, bool, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeValue{}, false, err
	}
	if n == nil {
		return RuntimeValue{}, false, nil
	}
	switch n.kind {
	case referenceArrayNode, referenceCollisionNode, referenceValueNode:
		for _, entry := range n.entries {
			equal, err := runtimeSourceCollectionEqualContext(ctx, key, entry.Key)
			if err != nil {
				return RuntimeValue{}, false, err
			}
			if equal {
				return entry.Value, true, nil
			}
		}
	case referenceBitmapNode:
		if err := runtimeMarkReferenceHashContext(ctx, key); err != nil {
			return RuntimeValue{}, false, err
		}
		bit := uint32(1) << ((hash >> shift) & 31)
		if n.bitmap&bit != 0 {
			return n.nodes[bits.OnesCount32(n.bitmap&(bit-1))].lookupContext(ctx, shift+5, hash, key)
		}
	case referenceDenseNode:
		if err := runtimeMarkReferenceHashContext(ctx, key); err != nil {
			return RuntimeValue{}, false, err
		}
		return n.nodes[(hash>>shift)&31].lookupContext(ctx, shift+5, hash, key)
	}
	return RuntimeValue{}, false, nil
}
func (n *runtimeReferenceMapNode) updateContext(ctx context.Context, shift uint, hash uint32, entry runtimeReferenceEntry, remove bool) (*runtimeReferenceMapNode, int, error) {
	if err := ctx.Err(); err != nil {
		return n, 0, err
	}
	if n == nil {
		if remove {
			return nil, 0, nil
		}
		return referenceValue(hash, entry), 1, nil
	}
	switch n.kind {
	case referenceArrayNode, referenceCollisionNode:
		if n.kind == referenceCollisionNode {
			if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
				return n, 0, err
			}
		}
		if n.kind == referenceCollisionNode && hash != n.hash {
			if remove {
				return n, 0, nil
			}
			return referenceMerge(n, shift, hash, entry), 1, nil
		}
		index := -1
		for i, current := range n.entries {
			equal, err := runtimeSourceCollectionEqualContext(ctx, entry.Key, current.Key)
			if err != nil {
				return n, 0, err
			}
			if equal {
				index = i
				break
			}
		}
		if index >= 0 && !remove && sameRuntimeObject(n.entries[index].Value, entry.Value) {
			return n, 0, nil
		}
		if index < 0 && remove {
			return n, 0, nil
		}
		if remove && len(n.entries) == 1 {
			return nil, -1, nil
		}
		if remove && n.kind == referenceCollisionNode && len(n.entries) == 2 {
			return referenceValue(n.hash, n.entries[index^1]), -1, nil
		}
		if index < 0 && !remove && n.kind == referenceArrayNode && len(n.entries) >= 8 {
			// Conversion inserts the ninth key first, then the original eight.
			if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
				return n, 0, err
			}
			root := referenceValue(hash, entry)
			for _, old := range n.entries {
				if err := runtimeMarkReferenceHashContext(ctx, old.Key); err != nil {
					return n, 0, err
				}
				h, err := runtimeReferenceHashContext(ctx, old.Key)
				if err != nil {
					return n, 0, err
				}
				root, _, err = root.updateContext(ctx, 0, uint32(h), old, false)
				if err != nil {
					return n, 0, err
				}
			}
			return root, 1, nil
		}
		copied := append([]runtimeReferenceEntry(nil), n.entries...)
		delta := 0
		if index >= 0 {
			if remove {
				copied[index] = copied[len(copied)-1]
				copied = copied[:len(copied)-1]
				delta = -1
			} else {
				copied[index] = entry
			}
		} else {
			copied = append(copied, entry)
			delta = 1
		}
		return &runtimeReferenceMapNode{kind: n.kind, hash: n.hash, entries: copied}, delta, nil
	case referenceValueNode:
		matches, err := runtimeSourceCollectionEqualContext(ctx, entry.Key, n.entries[0].Key)
		if err != nil {
			return n, 0, err
		}
		if remove {
			if matches {
				return nil, -1, nil
			}
			return n, 0, nil
		}
		if matches && sameRuntimeObject(n.entries[0].Value, entry.Value) {
			return n, 0, nil
		}
		if matches {
			return referenceValue(n.hash, entry), 0, nil
		}
		if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
			return n, 0, err
		}
		return referenceMerge(n, shift, hash, entry), 1, nil
	case referenceBitmapNode:
		if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
			return n, 0, err
		}
		fragment := (hash >> shift) & 31
		bit := uint32(1) << fragment
		exists := n.bitmap&bit != 0
		if !exists && remove {
			return n, 0, nil
		}
		index := bits.OnesCount32(n.bitmap & (bit - 1))
		var old *runtimeReferenceMapNode
		if exists {
			old = n.nodes[index]
		}
		child, delta, err := old.updateContext(ctx, shift+5, hash, entry, remove)
		if err != nil {
			return n, 0, err
		}
		if child == old {
			return n, 0, nil
		}
		if !exists && child != nil && len(n.nodes) >= 16 {
			dense := make([]*runtimeReferenceMapNode, 32)
			oldIndex := 0
			for i := 0; i < 32; i++ {
				if n.bitmap&(uint32(1)<<uint(i)) != 0 {
					dense[i] = n.nodes[oldIndex]
					oldIndex++
				}
			}
			dense[fragment] = child
			return &runtimeReferenceMapNode{kind: referenceDenseNode, count: len(n.nodes) + 1, nodes: dense}, delta, nil
		}
		if exists && child == nil && len(n.nodes) == 2 && referenceLeaf(n.nodes[index^1]) {
			return n.nodes[index^1], delta, nil
		}
		if exists && child != nil && len(n.nodes) == 1 && referenceLeaf(child) {
			return child, delta, nil
		}
		bitmap := n.bitmap
		children := append([]*runtimeReferenceMapNode(nil), n.nodes...)
		if exists {
			if child == nil {
				bitmap ^= bit
				children = append(children[:index], children[index+1:]...)
			} else {
				children[index] = child
			}
		} else {
			bitmap |= bit
			children = append(children, nil)
			copy(children[index+1:], children[index:])
			children[index] = child
		}
		if bitmap == 0 {
			return nil, delta, nil
		}
		return &runtimeReferenceMapNode{kind: referenceBitmapNode, bitmap: bitmap, nodes: children}, delta, nil
	case referenceDenseNode:
		if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
			return n, 0, err
		}
		index := (hash >> shift) & 31
		old := n.nodes[index]
		if old == nil && remove {
			return n, 0, nil
		}
		child, delta, err := old.updateContext(ctx, shift+5, hash, entry, remove)
		if err != nil {
			return n, 0, err
		}
		if child == old {
			return n, 0, nil
		}
		count := n.count
		if old == nil {
			count++
		} else if child == nil {
			count--
		}
		if old != nil && child == nil && count < 8 {
			bitmap := uint32(0)
			children := make([]*runtimeReferenceMapNode, 0, count)
			for i, node := range n.nodes {
				if uint32(i) != index && node != nil {
					bitmap |= uint32(1) << uint(i)
					children = append(children, node)
				}
			}
			if bitmap == 0 {
				return nil, delta, nil
			}
			return &runtimeReferenceMapNode{kind: referenceBitmapNode, bitmap: bitmap, nodes: children}, delta, nil
		}
		children := append([]*runtimeReferenceMapNode(nil), n.nodes...)
		children[index] = child
		return &runtimeReferenceMapNode{kind: referenceDenseNode, count: count, nodes: children}, delta, nil
	}
	return n, 0, fmt.Errorf("invalid reference map node")
}
func runtimeReferenceMapFromEntriesContext(ctx context.Context, entries []runtimeReferenceEntry) (runtimeReferenceMap, error) {
	return runtimeReferenceMapBatchSetContext(ctx, runtimeReferenceMap{}, entries)
}

// runtimeReferenceMapBatchSet inserts entries in a new withMutations owner batch.
// It preserves base and shares untouched immutable nodes. An editable ValueNode
// retains its existing key object when an equal key replaces its value; the
// first replacement of a borrowed leaf copies it with the incoming key instead.
// Array and collision nodes replace the key object on all non-noop updates.
func runtimeReferenceMapBatchSetContext(ctx context.Context, base runtimeReferenceMap, entries []runtimeReferenceEntry) (runtimeReferenceMap, error) {
	if err := ctx.Err(); err != nil {
		return runtimeReferenceMap{}, err
	}
	builder := referenceMapBuilder{owned: make(map[*runtimeReferenceMapNode]bool)}
	result := base
	for _, entry := range entries {
		if !entry.Value.Valid() {
			return runtimeReferenceMap{}, fmt.Errorf("reference map requires a valid runtime value")
		}
		hash, err := runtimeReferenceHashContext(ctx, entry.Key)
		if err != nil {
			return runtimeReferenceMap{}, err
		}
		if result.root == nil {
			result.root = builder.own(&runtimeReferenceMapNode{kind: referenceArrayNode, entries: []runtimeReferenceEntry{entry}})
			result.size = 1
			continue
		}
		var delta int
		result.root, delta, err = builder.insertContext(ctx, result.root, 0, uint32(hash), entry)
		if err != nil {
			return runtimeReferenceMap{}, err
		}
		result.size += delta
	}
	return result, nil
}

func (b *referenceMapBuilder) insertContext(ctx context.Context, node *runtimeReferenceMapNode, shift uint, hash uint32, entry runtimeReferenceEntry) (*runtimeReferenceMapNode, int, error) {
	if err := ctx.Err(); err != nil {
		return node, 0, err
	}
	if node == nil {
		return b.leaf(hash, entry), 1, nil
	}
	switch node.kind {
	case referenceArrayNode, referenceCollisionNode:
		if node.kind == referenceCollisionNode {
			if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
				return node, 0, err
			}
		}
		if node.kind == referenceCollisionNode && hash != node.hash {
			return b.merge(node, shift, hash, entry), 1, nil
		}
		for index, existing := range node.entries {
			equal, err := runtimeSourceCollectionEqualContext(ctx, entry.Key, existing.Key)
			if err != nil {
				return node, 0, err
			}
			if equal {
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
			if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
				return node, 0, err
			}
			root := b.leaf(hash, entry)
			for _, existing := range node.entries {
				if err := runtimeMarkReferenceHashContext(ctx, existing.Key); err != nil {
					return node, 0, err
				}
				existingHash, err := runtimeReferenceHashContext(ctx, existing.Key)
				if err != nil {
					return nil, 0, err
				}
				root, _, err = b.insertContext(ctx, root, 0, uint32(existingHash), existing)
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
		equal, err := runtimeSourceCollectionEqualContext(ctx, entry.Key, node.entries[0].Key)
		if err != nil {
			return node, 0, err
		}
		if equal {
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
		if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
			return node, 0, err
		}
		return b.merge(node, shift, hash, entry), 1, nil
	case referenceBitmapNode:
		if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
			return node, 0, err
		}
		fragment := (hash >> shift) & 31
		bit := uint32(1) << fragment
		exists := node.bitmap&bit != 0
		index := bits.OnesCount32(node.bitmap & (bit - 1))
		var old *runtimeReferenceMapNode
		if exists {
			old = node.nodes[index]
		}
		child, delta, err := b.insertContext(ctx, old, shift+5, hash, entry)
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
		if err := runtimeMarkReferenceHashContext(ctx, entry.Key); err != nil {
			return node, 0, err
		}
		index := (hash >> shift) & 31
		old := node.nodes[index]
		child, delta, err := b.insertContext(ctx, old, shift+5, hash, entry)
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
