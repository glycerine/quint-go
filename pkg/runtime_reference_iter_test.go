package quint

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// Retain the previous slice-materializing wrapper formula independently.
func runtimeIteratorAssembleOracle(op string, mapping runtimeReferenceMap) (RuntimeValue, error) {
	source := Expr{Kind: "app", Opcode: op, Args: []Expr{}}
	containsClosure := false
	referenceMemoKeys := false
	for _, entry := range mapping.Entries() {
		containsClosure = containsClosure || entry.Key.containsClosure || entry.Value.containsClosure
		referenceMemoKeys = referenceMemoKeys || runtimeKeyNeedsReferenceMemo(entry.Key)
		switch op {
		case "Set":
			source.Args = append(source.Args, entry.Value.source)
		case "Map":
			source.Args = append(source.Args, Expr{Kind: "app", Opcode: "Tup", Args: []Expr{entry.Key.source, entry.Value.source}})
		default:
			return RuntimeValue{}, fmt.Errorf("invalid retained collection %s", op)
		}
	}
	if op != "Set" && op != "Map" {
		return RuntimeValue{}, fmt.Errorf("invalid retained collection %s", op)
	}
	return runtimeAssemble(source, &runtimePayload{entries: mapping, containsClosure: containsClosure, referenceMemoKeys: referenceMemoKeys})
}

func runtimeIteratorValue(t testing.TB, number int64) RuntimeValue {
	t.Helper()
	integer, err := ParseInteger(fmt.Sprint(number))
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(Expr{Kind: "int", Int: integer})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func runtimeIteratorHistory(t testing.TB) []runtimeReferenceMap {
	t.Helper()
	history := []runtimeReferenceMap{{}}
	var current runtimeReferenceMap
	keys := make([]RuntimeValue, 0, 78)
	for i := 0; i < 78; i++ {
		number := int64(i)
		if i >= 65 {
			number = int64(i-64) * 4294967296
		}
		key, value := runtimeIteratorValue(t, number), runtimeIteratorValue(t, int64(i))
		var err error
		current, err = current.Set(key, value)
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
		history = append(history, current)
	}
	for i, key := range keys {
		var err error
		if i%3 == 0 {
			current, err = current.Set(key, runtimeIteratorValue(t, int64(-i)))
		} else {
			current, err = current.Delete(key)
		}
		if err != nil {
			t.Fatal(err)
		}
		history = append(history, current)
	}
	return history
}

func assertRuntimeIteratorEntries(t *testing.T, mapping runtimeReferenceMap) {
	t.Helper()
	// Entries is the independent previous traversal and retains its owned slice.
	want := mapping.Entries()
	root, size := mapping.root, mapping.size
	index := 0
	for entry := range mapping.all() {
		if index >= len(want) || !sameRuntimeObject(entry.Key, want[index].Key) || !sameRuntimeObject(entry.Value, want[index].Value) {
			t.Fatal("iterator changed entry order or retained object identity", index)
		}
		if entry.Key.payload != want[index].Key.payload || entry.Value.payload != want[index].Value.payload {
			t.Fatal("iterator rebuilt retained payloads")
		}
		index++
	}
	if index != len(want) || mapping.root != root || mapping.size != size {
		t.Fatal("iterator changed entry count or persistent map")
	}
}

func TestRuntimeReferenceIteratorHistory(t *testing.T) {
	t.Parallel()
	history := runtimeIteratorHistory(t)
	kinds := [5]bool{}
	var visit func(*runtimeReferenceMapNode)
	visit = func(node *runtimeReferenceMapNode) {
		if node == nil {
			return
		}
		kinds[node.kind] = true
		for _, child := range node.nodes {
			visit(child)
		}
	}
	for _, mapping := range history {
		assertRuntimeIteratorEntries(t, mapping)
		visit(mapping.root)
	}
	for kind, found := range kinds {
		if !found {
			t.Fatal("history did not cover tree node kind", kind)
		}
	}
}

func TestRuntimeReferenceIteratorBatchAndRetainedChildren(t *testing.T) {
	t.Parallel()
	children := runtimeSequenceCacheValues(t, false)
	entries := make([]runtimeReferenceEntry, 0, 35)
	for i := range 35 {
		entries = append(entries, runtimeReferenceEntry{runtimeIteratorValue(t, int64(i)), children[i%len(children)]})
	}
	mapping, err := runtimeReferenceMapFromEntries(entries)
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeIteratorEntries(t, mapping)
	updated, err := runtimeReferenceMapBatchSet(mapping, []runtimeReferenceEntry{entries[0], {entries[1].Key, children[0]}, {runtimeIteratorValue(t, 99), children[8]}})
	if err != nil {
		t.Fatal(err)
	}
	assertRuntimeIteratorEntries(t, updated)
	assertRuntimeIteratorEntries(t, mapping)
	for entry := range mapping.all() {
		entry.Value = RuntimeValue{}
		break
	}
	assertRuntimeIteratorEntries(t, mapping)
}

func TestRuntimeReferenceIteratorEarlyStopAndPersistentUpdate(t *testing.T) {
	t.Parallel()
	history := runtimeIteratorHistory(t)
	mapping := history[78]
	want := mapping.Entries()
	for stop := 1; stop <= len(want); stop++ {
		seen := 0
		mapping.all()(func(entry runtimeReferenceEntry) bool {
			if !sameRuntimeObject(entry.Key, want[seen].Key) {
				t.Fatal("early-stop traversal changed order")
			}
			seen++
			return seen < stop
		})
		if seen != stop {
			t.Fatal("iterator continued after consumer stopped", seen, stop)
		}
	}
	sequence := mapping.all()
	var err error
	mapping, err = mapping.Delete(want[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for entry := range sequence {
		if !sameRuntimeObject(entry.Key, want[seen].Key) {
			t.Fatal("captured iterator changed after persistent wrapper update")
		}
		seen++
	}
	if seen != len(want) || mapping.Len() != len(want)-1 {
		t.Fatal("persistent iterator snapshot changed")
	}
	for range (runtimeReferenceMap{}).all() {
		t.Fatal("empty iterator yielded an entry")
	}
}

func TestRuntimeReferenceIteratorConcurrentReads(t *testing.T) {
	t.Parallel()
	mapping := runtimeIteratorHistory(t)[78]
	want := mapping.Entries()
	sequence := mapping.all()
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for range 5 {
				seen := 0
				for entry := range sequence {
					if !sameRuntimeObject(entry.Key, want[seen].Key) || !sameRuntimeObject(entry.Value, want[seen].Value) {
						t.Error("concurrent iteration changed retained entry")
						return
					}
					seen++
				}
				if seen != len(want) {
					t.Error("concurrent iterator changed size")
				}
			}
		})
	}
	workers.Wait()
}

func TestRuntimeReferenceIteratorAssemble(t *testing.T) {
	t.Parallel()
	history := runtimeIteratorHistory(t)
	mappings := []runtimeReferenceMap{}
	for _, index := range []int{0, 8, 9, 17, 32, 65, 78, len(history) - 1} {
		mappings = append(mappings, history[index])
	}
	children := runtimeSequenceCacheValues(t, false)
	children = append(children, newRuntimeClosure(0, nil))
	var retained runtimeReferenceMap
	for i, child := range children {
		var err error
		retained, err = retained.Set(runtimeIteratorValue(t, int64(i)), child)
		if err != nil {
			t.Fatal(err)
		}
	}
	mappings = append(mappings, retained)
	retained, err := retained.Set(children[9], children[8])
	if err != nil {
		t.Fatal(err)
	}
	mappings = append(mappings, retained)
	for index, mapping := range mappings {
		for _, op := range []string{"Set", "Map", "invalid"} {
			got, err := runtimeWithEntries(op, mapping)
			want, wantErr := runtimeIteratorAssembleOracle(op, mapping)
			if (err == nil) != (wantErr == nil) || fmt.Sprint(err) != fmt.Sprint(wantErr) {
				t.Fatal("retained traversal changed wrapper validation", op, index, err, wantErr)
			}
			if err != nil {
				continue
			}
			if !reflect.DeepEqual(got.expression, want.expression) || !reflect.DeepEqual(got.source, want.source) || got.identity != want.identity || got.allocation == nil || got.allocation == want.allocation || got.payload.entries.root != mapping.root || want.payload.entries.root != mapping.root || got.payload.referenceMemoKeys != want.payload.referenceMemoKeys {
				t.Fatal("retained traversal changed source/cache/key/identity/tree history", op, index)
			}
			for _, start := range []uint32{0, 0x3ffffffd} {
				a, b := &RuntimeObjectHashState{}, &RuntimeObjectHashState{}
				_ = a.Restore(start)
				_ = b.Restore(start)
				x, e1 := SerializeRuntimeValueContext(context.Background(), got, a)
				y, e2 := SerializeRuntimeValueContext(context.Background(), want, b)
				if fmt.Sprint(e1) != fmt.Sprint(e2) || !reflect.DeepEqual(x, y) || a.Counter() != b.Counter() {
					t.Fatal("retained traversal changed serialization/HAMT/UID history", op, index, start, e1, e2)
				}
			}
		}
	}
}
