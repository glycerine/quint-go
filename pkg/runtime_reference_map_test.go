package quint

import (
	"encoding/json"
	"os"
	"testing"
)

type referenceMapFixtureEntry struct {
	Key, Value           Expr
	KeyOrder, ValueOrder []Expr
}
type referenceMapFixtureStep struct {
	Kind       string
	Key        Expr
	Value      *Expr
	Found      bool
	ReuseValue bool
	Root       string
	Entries    []referenceMapFixtureEntry
}

func TestUpstreamReferenceMapHistory(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name  string
		Steps []referenceMapFixtureStep
	}
	data, err := os.ReadFile("testdata/parser/runtime_reference_map.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	value := func(t *testing.T, e Expr) RuntimeValue {
		t.Helper()
		v, err := NewRuntimeValue(e)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			var current runtimeReferenceMap
			history := []runtimeReferenceMap{}
			assertMap := func(m runtimeReferenceMap, step referenceMapFixtureStep) {
				t.Helper()
				kinds := []string{"ArrayMapNode", "ValueNode", "HashCollisionNode", "BitmapIndexedNode", "HashArrayMapNode"}
				root := "empty"
				if m.root != nil {
					root = kinds[m.root.kind]
				}
				if root != step.Root {
					t.Fatal("root", root, step.Root)
				}
				entries := m.Entries()
				if m.Len() != len(step.Entries) || len(entries) != len(step.Entries) {
					t.Fatal("size", m.Len(), len(entries), len(step.Entries))
				}
				order := func(v RuntimeValue, want []Expr) {
					t.Helper()
					if want == nil {
						return
					}
					if len(v.source.Args) != len(want) {
						t.Fatal("nested map size")
					}
					for i, key := range want {
						if !value(t, v.source.Args[i].Args[0]).Equal(value(t, key)) {
							t.Fatal("nested map key order", i)
						}
					}
				}
				for i, want := range step.Entries {
					if !entries[i].Key.Equal(value(t, want.Key)) || !entries[i].Value.Equal(value(t, want.Value)) {
						t.Fatal("entry", i, entries[i].Key.String(), entries[i].Value.String(), ExpressionString(want.Key), ExpressionString(want.Value))
					}
					order(entries[i].Key, want.KeyOrder)
					order(entries[i].Value, want.ValueOrder)
					got, found, err := m.Lookup(entries[i].Key)
					if err != nil || !found || !got.Equal(entries[i].Value) {
						t.Fatal("lookup entry", i, err, found)
					}
				}
			}
			for index, step := range fixture.Steps {
				key := value(t, step.Key)
				switch step.Kind {
				case "set":
					incoming := RuntimeValue{}
					if step.ReuseValue {
						var found bool
						incoming, found, err = current.Lookup(key)
						if err != nil || !found {
							t.Fatal("missing reuse value", err)
						}
					} else {
						incoming = value(t, *step.Value)
					}
					prior := current.root
					current, err = current.Set(key, incoming)
					if step.ReuseValue && current.root != prior {
						t.Fatal("same-object update changed root")
					}
				case "delete":
					current, err = current.Delete(key)
				case "lookup":
					var got RuntimeValue
					var found bool
					got, found, err = current.Lookup(key)
					if found != step.Found || (found && !got.Equal(value(t, *step.Value))) {
						t.Fatal(index, "lookup", found, step.Found, got.String())
					}
				}
				if err != nil {
					t.Fatal(index, err)
				}
				assertMap(current, step)
				history = append(history, current)
			}
			for i, m := range history {
				assertMap(m, fixture.Steps[i])
			}
		})
	}
}

func TestReferenceMapValidationOwnershipAndMissingDeletion(t *testing.T) {
	value, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	var empty runtimeReferenceMap
	for _, key := range []RuntimeValue{{}, {identity: "unsupported", expression: Expr{Kind: "name", Name: "x"}}} {
		if _, err := empty.Set(key, value); err == nil {
			t.Fatal("invalid key set")
		}
		if _, err := empty.Delete(key); err == nil {
			t.Fatal("invalid key delete")
		}
		if _, _, err := empty.Lookup(key); err == nil {
			t.Fatal("invalid key lookup")
		}
	}
	if _, err := empty.Set(value, RuntimeValue{}); err == nil {
		t.Fatal("invalid value")
	}
	m, err := empty.Set(value, value)
	if err != nil {
		t.Fatal(err)
	}
	entries := m.Entries()
	entries[0] = runtimeReferenceEntry{}
	if m.Len() != 1 || empty.Len() != 0 || !m.Entries()[0].Key.Equal(value) {
		t.Fatal("map aliases entry slice or changed parent")
	}
	missing, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: false})
	unchanged, err := m.Delete(missing)
	if err != nil || unchanged.root != m.root {
		t.Fatal("missing deletion changed root", err)
	}
	removed, err := m.Delete(value)
	if err != nil || removed.Len() != 0 || removed.root != nil || m.Len() != 1 {
		t.Fatal("delete changed original", err)
	}
}

func TestRuntimeAllocationIdentity(t *testing.T) {
	first, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	second, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	if !first.Equal(second) || first.Hash() != second.Hash() || sameRuntimeObject(first, second) || !sameRuntimeObject(first, first) {
		t.Fatal("allocation identity changed structural semantics")
	}
	if sameRuntimeObject(RuntimeValue{}, RuntimeValue{}) {
		t.Fatal("invalid values share identity")
	}
	closure := newRuntimeClosure(0, nil)
	if !sameRuntimeObject(closure, closure) || sameRuntimeObject(closure, newRuntimeClosure(0, nil)) {
		t.Fatal("closure identity")
	}
}
