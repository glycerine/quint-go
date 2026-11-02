package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamReferenceMapBuild(t *testing.T) {
	t.Parallel()
	type inputEntry struct {
		Key        Expr
		Value      *Expr
		ReuseIndex json.RawMessage
	}
	type outputEntry struct {
		Key, Value           Expr
		KeyOrder, ValueOrder []Expr
	}
	type batch struct {
		Entries []inputEntry
		Root    string
		Output  []outputEntry
	}
	var fixtures []struct {
		Name    string
		Batches []batch
	}
	data, err := os.ReadFile("testdata/parser/runtime_reference_map_build.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	value := func(t *testing.T, expression Expr) RuntimeValue {
		t.Helper()
		v, err := NewRuntimeValue(expression)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			var current runtimeReferenceMap
			var history []runtimeReferenceMap
			var values [][]RuntimeValue
			assertMap := func(m runtimeReferenceMap, want batch) {
				t.Helper()
				root := "empty"
				if m.root != nil {
					root = []string{"ArrayMapNode", "ValueNode", "HashCollisionNode", "BitmapIndexedNode", "HashArrayMapNode"}[m.root.kind]
				}
				if root != want.Root {
					t.Fatal("root", root, want.Root)
				}
				entries := m.Entries()
				if m.Len() != len(want.Output) || len(entries) != len(want.Output) {
					t.Fatal("size", m.Len(), len(entries), len(want.Output))
				}
				order := func(v RuntimeValue, want []Expr) {
					t.Helper()
					if want == nil {
						return
					}
					if len(v.source.Args) != len(want) {
						t.Fatal("nested map length")
					}
					for i, key := range want {
						if !value(t, v.source.Args[i].Args[0]).Equal(value(t, key)) {
							t.Fatal("nested key order", i, v.String())
						}
					}
				}
				for i, want := range want.Output {
					if !entries[i].Key.Equal(value(t, want.Key)) || !entries[i].Value.Equal(value(t, want.Value)) {
						t.Fatal("entry", i, entries[i].Key.String(), entries[i].Value.String())
					}
					order(entries[i].Key, want.KeyOrder)
					order(entries[i].Value, want.ValueOrder)
					got, found, err := m.Lookup(entries[i].Key)
					if err != nil || !found || !got.Equal(entries[i].Value) {
						t.Fatal("lookup", i, got.String(), found, err)
					}
				}
			}
			for index, batch := range fixture.Batches {
				entries := make([]runtimeReferenceEntry, 0, len(batch.Entries))
				var batchValues []RuntimeValue
				for _, input := range batch.Entries {
					key := value(t, input.Key)
					var v RuntimeValue
					if len(input.ReuseIndex) == 0 {
						v = value(t, *input.Value)
					} else if input.ReuseIndex[0] == '{' {
						var reused struct{ Batch, Index int }
						if err = json.Unmarshal(input.ReuseIndex, &reused); err != nil {
							t.Fatal(err)
						}
						v = values[reused.Batch][reused.Index]
					} else {
						var reused int
						if err = json.Unmarshal(input.ReuseIndex, &reused); err != nil {
							t.Fatal(err)
						}
						v = batchValues[reused]
					}
					batchValues = append(batchValues, v)
					entries = append(entries, runtimeReferenceEntry{key, v})
				}
				values = append(values, batchValues)
				if index == 0 {
					current, err = runtimeReferenceMapFromEntries(entries)
				} else {
					current, err = runtimeReferenceMapBatchSet(current, entries)
				}
				if err != nil {
					t.Fatal(err)
				}
				assertMap(current, batch)
				history = append(history, current)
			}
			for index, m := range history {
				assertMap(m, fixture.Batches[index])
			}
		})
	}
}

func TestReferenceMapBuildValidationAndOwnership(t *testing.T) {
	key, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	other, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: false})
	input := []runtimeReferenceEntry{{key, other}}
	base, err := runtimeReferenceMapFromEntries(input)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = runtimeReferenceEntry{}
	if base.Len() != 1 || !base.Entries()[0].Value.Equal(other) {
		t.Fatal("input slice aliased")
	}
	unchanged, err := runtimeReferenceMapBatchSet(base, []runtimeReferenceEntry{{key, other}})
	if err != nil || unchanged.root != base.root {
		t.Fatal("same-object batch changed root", err)
	}
	empty, err := runtimeReferenceMapFromEntries(nil)
	if err != nil || empty.Len() != 0 || empty.root != nil {
		t.Fatal("empty constructor", err)
	}
	for _, entry := range []runtimeReferenceEntry{{RuntimeValue{}, other}, {newRuntimeClosure(0, nil), other}, {key, RuntimeValue{}}} {
		if _, err := runtimeReferenceMapFromEntries([]runtimeReferenceEntry{entry}); err == nil {
			t.Fatal("invalid constructor input")
		}
		if _, err := runtimeReferenceMapBatchSet(base, []runtimeReferenceEntry{{key, key}, entry}); err == nil {
			t.Fatal("invalid batch input")
		}
		if !base.Entries()[0].Value.Equal(other) {
			t.Fatal("failed batch modified base")
		}
	}
	updated, err := base.Set(key, key)
	if err != nil || !updated.Entries()[0].Value.Equal(key) || !base.Entries()[0].Value.Equal(other) {
		t.Fatal("post-constructor persistent update changed base", err)
	}
}
