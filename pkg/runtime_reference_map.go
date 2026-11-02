package quint

import (
	"fmt"
	"math/bits"
)

// runtimeReferenceMap retains the immutable node history that determines
// Immutable.js Map traversal. The zero value is an empty map. Keys use the
// Go runtime's structural equality boundary and the private reference hash.
// Nodes and slices are never mutated after publication. Updates reproduce the
// persistent ownerless path; mutable ownerID batches used by some upstream
// constructors require a separate transient API before they can be reproduced.
type runtimeReferenceMap struct {
	root *runtimeReferenceMapNode
	size int
}

type runtimeReferenceEntry struct{ Key, Value RuntimeValue }
type runtimeReferenceMapNode struct {
	kind    uint8
	hash    uint32
	bitmap  uint32
	count   int
	entries []runtimeReferenceEntry
	nodes   []*runtimeReferenceMapNode
}

const (
	referenceArrayNode uint8 = iota
	referenceValueNode
	referenceCollisionNode
	referenceBitmapNode
	referenceDenseNode
)

func (m runtimeReferenceMap) Len() int { return m.size }
func (m runtimeReferenceMap) Lookup(key RuntimeValue) (RuntimeValue, bool, error) {
	hash, err := runtimeReferenceHash(key)
	if err != nil {
		return RuntimeValue{}, false, err
	}
	value, found := m.root.lookup(0, uint32(hash), key)
	return value, found, nil
}
func (m runtimeReferenceMap) Set(key, value RuntimeValue) (runtimeReferenceMap, error) {
	if !value.Valid() {
		return m, fmt.Errorf("reference map requires a valid runtime value")
	}
	hash, err := runtimeReferenceHash(key)
	if err != nil {
		return m, err
	}
	entry := runtimeReferenceEntry{key, value}
	if m.root == nil {
		return runtimeReferenceMap{root: &runtimeReferenceMapNode{kind: referenceArrayNode, entries: []runtimeReferenceEntry{entry}}, size: 1}, nil
	}
	root, delta, err := m.root.update(0, uint32(hash), entry, false)
	if err != nil {
		return m, err
	}
	return runtimeReferenceMap{root: root, size: m.size + delta}, nil
}
func (m runtimeReferenceMap) Delete(key RuntimeValue) (runtimeReferenceMap, error) {
	hash, err := runtimeReferenceHash(key)
	if err != nil {
		return m, err
	}
	if m.root == nil {
		return m, nil
	}
	root, delta, err := m.root.update(0, uint32(hash), runtimeReferenceEntry{Key: key}, true)
	if err != nil {
		return m, err
	}
	return runtimeReferenceMap{root: root, size: m.size + delta}, nil
}
func (m runtimeReferenceMap) Entries() []runtimeReferenceEntry {
	entries := make([]runtimeReferenceEntry, 0, m.size)
	var visit func(*runtimeReferenceMapNode)
	visit = func(node *runtimeReferenceMapNode) {
		if node == nil {
			return
		}
		switch node.kind {
		case referenceArrayNode, referenceCollisionNode, referenceValueNode:
			entries = append(entries, node.entries...)
		default:
			for _, child := range node.nodes {
				visit(child)
			}
		}
	}
	visit(m.root)
	return entries
}
func (n *runtimeReferenceMapNode) lookup(shift uint, hash uint32, key RuntimeValue) (RuntimeValue, bool) {
	if n == nil {
		return RuntimeValue{}, false
	}
	switch n.kind {
	case referenceArrayNode, referenceCollisionNode, referenceValueNode:
		for _, entry := range n.entries {
			if entry.Key.Equal(key) {
				return entry.Value, true
			}
		}
	case referenceBitmapNode:
		bit := uint32(1) << ((hash >> shift) & 31)
		if n.bitmap&bit != 0 {
			return n.nodes[bits.OnesCount32(n.bitmap&(bit-1))].lookup(shift+5, hash, key)
		}
	case referenceDenseNode:
		return n.nodes[(hash>>shift)&31].lookup(shift+5, hash, key)
	}
	return RuntimeValue{}, false
}
func referenceLeaf(n *runtimeReferenceMapNode) bool {
	return n != nil && (n.kind == referenceValueNode || n.kind == referenceCollisionNode)
}
func referenceValue(hash uint32, entry runtimeReferenceEntry) *runtimeReferenceMapNode {
	return &runtimeReferenceMapNode{kind: referenceValueNode, hash: hash, entries: []runtimeReferenceEntry{entry}}
}
func referenceMerge(node *runtimeReferenceMapNode, shift uint, hash uint32, entry runtimeReferenceEntry) *runtimeReferenceMapNode {
	if node.hash == hash {
		return &runtimeReferenceMapNode{kind: referenceCollisionNode, hash: hash, entries: []runtimeReferenceEntry{node.entries[0], entry}}
	}
	first, second := (node.hash>>shift)&31, (hash>>shift)&31
	bitmap := (uint32(1) << first) | (uint32(1) << second)
	children := []*runtimeReferenceMapNode{}
	if first == second {
		children = append(children, referenceMerge(node, shift+5, hash, entry))
	} else if first < second {
		children = append(children, node, referenceValue(hash, entry))
	} else {
		children = append(children, referenceValue(hash, entry), node)
	}
	return &runtimeReferenceMapNode{kind: referenceBitmapNode, bitmap: bitmap, nodes: children}
}
func (n *runtimeReferenceMapNode) update(shift uint, hash uint32, entry runtimeReferenceEntry, remove bool) (*runtimeReferenceMapNode, int, error) {
	if n == nil {
		if remove {
			return nil, 0, nil
		}
		return referenceValue(hash, entry), 1, nil
	}
	switch n.kind {
	case referenceArrayNode, referenceCollisionNode:
		if n.kind == referenceCollisionNode && hash != n.hash {
			if remove {
				return n, 0, nil
			}
			return referenceMerge(n, shift, hash, entry), 1, nil
		}
		index := -1
		for i, current := range n.entries {
			if current.Key.Equal(entry.Key) {
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
			root := referenceValue(hash, entry)
			for _, old := range n.entries {
				h, err := runtimeReferenceHash(old.Key)
				if err != nil {
					return n, 0, err
				}
				root, _, err = root.update(0, uint32(h), old, false)
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
		matches := n.entries[0].Key.Equal(entry.Key)
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
		return referenceMerge(n, shift, hash, entry), 1, nil
	case referenceBitmapNode:
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
		child, delta, err := old.update(shift+5, hash, entry, remove)
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
		index := (hash >> shift) & 31
		old := n.nodes[index]
		if old == nil && remove {
			return n, 0, nil
		}
		child, delta, err := old.update(shift+5, hash, entry, remove)
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
