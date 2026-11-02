package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"testing"
)

func TestUpstreamContextReferenceMapHistory(t *testing.T) {
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
					got, found, err := m.LookupContext(context.Background(), entries[i].Key)
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
						incoming, found, err = current.LookupContext(context.Background(), key)
						if err != nil || !found {
							t.Fatal("missing reuse value", err)
						}
					} else {
						incoming = value(t, *step.Value)
					}
					prior := current.root
					current, err = current.SetContext(context.Background(), key, incoming)
					if step.ReuseValue && current.root != prior {
						t.Fatal("same-object update changed root")
					}
				case "delete":
					current, err = current.DeleteContext(context.Background(), key)
				case "lookup":
					var got RuntimeValue
					var found bool
					got, found, err = current.LookupContext(context.Background(), key)
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

func TestUpstreamContextReferenceMapBuild(t *testing.T) {
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
					got, found, err := m.LookupContext(context.Background(), entries[i].Key)
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
					current, err = runtimeReferenceMapFromEntriesContext(context.Background(), entries)
				} else {
					current, err = runtimeReferenceMapBatchSetContext(context.Background(), current, entries)
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

func TestContextReferenceMapBuildValidationAndOwnership(t *testing.T) {
	key, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	other, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: false})
	input := []runtimeReferenceEntry{{key, other}}
	base, err := runtimeReferenceMapFromEntriesContext(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	input[0] = runtimeReferenceEntry{}
	if base.Len() != 1 || !base.Entries()[0].Value.Equal(other) {
		t.Fatal("input slice aliased")
	}
	unchanged, err := runtimeReferenceMapBatchSetContext(context.Background(), base, []runtimeReferenceEntry{{key, other}})
	if err != nil || unchanged.root != base.root {
		t.Fatal("same-object batch changed root", err)
	}
	empty, err := runtimeReferenceMapFromEntriesContext(context.Background(), nil)
	if err != nil || empty.Len() != 0 || empty.root != nil {
		t.Fatal("empty constructor", err)
	}
	for _, entry := range []runtimeReferenceEntry{{RuntimeValue{}, other}, {RuntimeValue{identity: "unsupported", expression: Expr{Kind: "name", Name: "x"}}, other}, {key, RuntimeValue{}}} {
		if _, err := runtimeReferenceMapFromEntriesContext(context.Background(), []runtimeReferenceEntry{entry}); err == nil {
			t.Fatal("invalid constructor input")
		}
		if _, err := runtimeReferenceMapBatchSetContext(context.Background(), base, []runtimeReferenceEntry{{key, key}, entry}); err == nil {
			t.Fatal("invalid batch input")
		}
		if !base.Entries()[0].Value.Equal(other) {
			t.Fatal("failed batch modified base")
		}
	}
	updated, err := base.SetContext(context.Background(), key, key)
	if err != nil || !updated.Entries()[0].Value.Equal(key) || !base.Entries()[0].Value.Equal(other) {
		t.Fatal("post-constructor persistent update changed base", err)
	}
}

func TestContextReferenceMapSymbolicArrayKeys(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	interval := contextIdentityDomain(t, NewRuntimeInterval(domainFixtureInteger(t, "1"), domainFixtureInteger(t, "3")))
	explicit := contextIdentityValue(t, "Set(1,2,3)")
	label := contextIdentityValue(t, `"D"`)
	left := contextIdentityComposite(t, "variant", label, interval)
	right := contextIdentityComposite(t, "variant", label, explicit)
	first, next := contextIdentityValue(t, "0"), contextIdentityValue(t, "1")
	leftHash, _ := runtimeReferenceHashContext(ctx, left)
	rightHash, _ := runtimeReferenceHashContext(ctx, right)
	if leftHash == rightHash {
		t.Fatal("fixture requires different raw reference hashes")
	}
	for _, keys := range [][2]RuntimeValue{{interval, explicit}, {left, right}} {
		var m runtimeReferenceMap
		m, err := m.SetContext(ctx, keys[0], first)
		if err != nil {
			t.Fatal(err)
		}
		found, ok, err := m.LookupContext(ctx, keys[1])
		if err != nil || !ok || !sameRuntimeObject(found, first) {
			t.Fatal("array lookup should compare checked equality before hashes", ok, err)
		}
		unchanged, err := m.SetContext(ctx, keys[1], first)
		if err != nil || unchanged.root != m.root || !sameRuntimeObject(unchanged.Entries()[0].Key, keys[0]) {
			t.Fatal("same-object update changed key or root", err)
		}
		updated, err := m.SetContext(ctx, keys[1], next)
		if err != nil || updated.Len() != 1 || !sameRuntimeObject(updated.Entries()[0].Key, keys[1]) || !sameRuntimeObject(m.Entries()[0].Value, first) {
			t.Fatal("array replacement must keep incoming key and immutable base", err)
		}
		removed, err := updated.DeleteContext(ctx, keys[0])
		if err != nil || removed.Len() != 0 || removed.root != nil {
			t.Fatal("checked array removal", err)
		}
		batch, err := runtimeReferenceMapFromEntriesContext(ctx, []runtimeReferenceEntry{{keys[0], first}, {keys[1], next}})
		if err != nil || batch.Len() != 1 || !sameRuntimeObject(batch.Entries()[0].Key, keys[1]) {
			t.Fatal("transient array equality must retain last incoming key", err)
		}
	}
}

func TestContextReferenceMapEqualityErrorsAndObjectIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	integers, _ := NewRuntimeInfiniteSet("Int")
	rangeSet, _ := RuntimeDomainFromValue(contextIdentityValue(t, "Set(0)"))
	mapset, _ := NewRuntimeMapSet(integers, rangeSet)
	powerset, _ := NewRuntimePowerset(integers)
	stored, incoming := contextIdentityDomain(t, mapset), contextIdentityDomain(t, powerset)
	value := contextIdentityValue(t, "0")
	base, err := (runtimeReferenceMap{}).SetContext(ctx, stored, value)
	if err != nil {
		t.Fatal(err)
	}
	// Comparing the incoming powerset first throws its own expansion error;
	// comparing the stored mapset first would throw a different exception.
	assertError := func(err error) {
		t.Helper()
		var exception RuntimeDomainException
		if !errors.As(err, &exception) || exception.Name != "Error" || exception.Message != "Either state is Left" {
			t.Fatal("incoming key equality error changed", err)
		}
	}
	_, _, err = base.LookupContext(ctx, incoming)
	assertError(err)
	updated, err := base.SetContext(ctx, incoming, value)
	assertError(err)
	if updated.root != base.root || updated.Len() != base.Len() {
		t.Fatal("failed update changed persistent base")
	}
	removed, err := base.DeleteContext(ctx, incoming)
	assertError(err)
	if removed.root != base.root || removed.Len() != base.Len() {
		t.Fatal("failed deletion changed persistent base")
	}
	_, err = runtimeReferenceMapBatchSetContext(ctx, base, []runtimeReferenceEntry{{incoming, value}})
	assertError(err)
	if got, found, err := base.LookupContext(ctx, stored); err != nil || !found || !sameRuntimeObject(got, value) {
		t.Fatal("same-object shortcut should avoid failing domain expansion", found, err)
	}
	unchanged, err := base.SetContext(ctx, stored, value)
	if err != nil || unchanged.root != base.root {
		t.Fatal("same-object key/value update should be a no-op", err)
	}
	zero := contextIdentityValue(t, "0")
	base, err = base.SetContext(ctx, zero, value)
	if err != nil {
		t.Fatal(err)
	}
	_, err = runtimeReferenceMapBatchSetContext(ctx, base, []runtimeReferenceEntry{{zero, contextIdentityValue(t, "1")}, {incoming, value}})
	assertError(err)
	if got, found, err := base.LookupContext(ctx, zero); err != nil || !found || !sameRuntimeObject(got, value) {
		t.Fatal("failed batch mutated borrowed node", found, err)
	}
}

func TestContextReferenceMapCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	key, value := contextIdentityValue(t, "0"), contextIdentityValue(t, "1")
	base, _ := (runtimeReferenceMap{}).SetContext(context.Background(), key, value)
	for _, call := range []func() error{
		func() error { _, _, err := base.LookupContext(ctx, key); return err },
		func() error { _, err := base.SetContext(ctx, key, value); return err },
		func() error { _, err := base.DeleteContext(ctx, key); return err },
		func() error { _, err := runtimeReferenceMapFromEntriesContext(ctx, nil); return err },
		func() error { _, err := runtimeReferenceMapBatchSetContext(ctx, base, nil); return err },
	} {
		if err := call(); !errors.Is(err, context.Canceled) {
			t.Fatal("pre-cancellation ignored", err)
		}
	}
	entries := make([]runtimeReferenceEntry, 64)
	for i := range entries {
		entries[i] = runtimeReferenceEntry{Key: contextIdentityValue(t, "0"), Value: contextIdentityValue(t, "1")}
	}
	during := &contextIdentityCancelContext{Context: context.Background()}
	if _, err := runtimeReferenceMapBatchSetContext(during, base, entries); !errors.Is(err, context.Canceled) {
		t.Fatal("batch did not propagate cancellation during mutations", err)
	}
	if got, found, err := base.LookupContext(context.Background(), key); err != nil || !found || !sameRuntimeObject(got, value) {
		t.Fatal("canceled batch mutated base", found, err)
	}
}

func TestContextReferenceMapValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	key, value := contextIdentityValue(t, "0"), contextIdentityValue(t, "1")
	base, err := (runtimeReferenceMap{}).SetContext(ctx, key, value)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []func() error{
		func() error { _, _, err := base.LookupContext(ctx, RuntimeValue{}); return err },
		func() error { _, err := base.SetContext(ctx, RuntimeValue{}, value); return err },
		func() error { _, err := base.SetContext(ctx, key, RuntimeValue{}); return err },
		func() error { _, err := base.DeleteContext(ctx, RuntimeValue{}); return err },
	} {
		if err := call(); err == nil {
			t.Fatal("invalid runtime map input accepted")
		}
	}
	if got, found, err := base.LookupContext(ctx, key); err != nil || !found || !sameRuntimeObject(got, value) {
		t.Fatal("failed validation changed retained value", found, err)
	}
}

type contextReferenceEqualityObservation struct {
	AToDuplicate bool
	BToDuplicate bool
	DuplicateToA bool
	DuplicateToB bool
	AToB         bool
}

func TestUpstreamContextReferenceMapHashHistory(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Cases []struct {
			Name                   string
			ConcreteComparisonMaps bool
			Hashes                 []float64
			Before, After          struct {
				Cached   []bool
				Equality contextReferenceEqualityObservation
			}
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_symbolic_hash_pending.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			zero := contextIdentityValue(t, "0")
			label := contextIdentityValue(t, `"X"`)
			intervalDomain := NewRuntimeInterval(domainFixtureInteger(t, "-10"), domainFixtureInteger(t, "8"))
			interval := contextIdentityDomain(t, intervalDomain)
			explicit, err := intervalDomain.Materialize(ctx)
			if err != nil {
				t.Fatal(err)
			}
			intervalKey := contextIdentityComposite(t, "variant", label, interval)
			explicitKey := contextIdentityComposite(t, "variant", label, explicit)
			integer := func(n int) Integer {
				return domainFixtureInteger(t, strconv.Itoa(n))
			}
			singleton := func(n int) RuntimeValue {
				var payload RuntimeValue
				if test.ConcreteComparisonMaps {
					value, err := NewRuntimeValue(Expr{Kind: "int", Int: integer(n)})
					if err != nil {
						t.Fatal(err)
					}
					payload = contextIdentityComposite(t, "Set", value)
				} else {
					payload = contextIdentityDomain(t, NewRuntimeInterval(integer(n), integer(n)))
				}
				return contextIdentityComposite(t, "variant", label, payload)
			}
			prefix := contextIdentityComposite(t, "variant", label, contextIdentityDomain(t, NewRuntimeInterval(integer(7), integer(7))))
			fillers := make([]RuntimeValue, 7)
			for i := range fillers {
				fillers[i] = singleton(i)
			}
			makeMap := func(keys []RuntimeValue) RuntimeValue {
				pairs := make([]RuntimeValue, len(keys))
				for i, key := range keys {
					pairs[i] = contextIdentityComposite(t, "Tup", key, zero)
				}
				return contextIdentityComposite(t, "Map", pairs...)
			}
			initial := makeMap(append([]RuntimeValue{intervalKey, prefix}, fillers...))
			raw, err := runtimeMap(initial)
			if err != nil {
				t.Fatal(err)
			}
			raw, err = raw.SetContext(ctx, explicitKey, zero)
			if err != nil {
				t.Fatal(err)
			}
			raw, err = raw.DeleteContext(ctx, prefix)
			if err != nil {
				t.Fatal(err)
			}
			duplicate, err := runtimeWithEntriesContext(ctx, "Map", raw)
			if err != nil || raw.Len() != 9 {
				t.Fatal("duplicate-key fixture construction", raw.Len(), err)
			}
			extra := 8
			if test.ConcreteComparisonMaps {
				extra = 7
			}
			a := makeMap(append(append([]RuntimeValue{explicitKey}, fillers...), singleton(extra)))
			b := makeMap(append(append([]RuntimeValue{explicitKey}, fillers...), singleton(9)))
			if test.ConcreteComparisonMaps && (a.containsDomain || b.containsDomain) {
				t.Fatal("comparison maps must be fully concrete")
			}
			observe := func() contextReferenceEqualityObservation {
				check := func(left, right RuntimeValue) bool {
					value, err := runtimeSourceEqualContext(ctx, left, right)
					if err != nil {
						t.Fatal(err)
					}
					return value
				}
				return contextReferenceEqualityObservation{
					check(a, duplicate), check(b, duplicate), check(duplicate, a), check(duplicate, b), check(a, b),
				}
			}
			if got := observe(); got != test.Before.Equality {
				t.Fatal("source equality before hashing", got, test.Before.Equality)
			}
			checkPublic := func() {
				t.Helper()
				for _, pair := range [][2]RuntimeValue{{a, duplicate}, {duplicate, a}, {b, duplicate}, {duplicate, b}, {a, b}, {b, a}} {
					if equal, err := pair[0].EqualContext(ctx, pair[1]); err != nil || equal {
						t.Fatal("public equality must distinguish different entry bags symmetrically", equal, err)
					}
				}
				aHash, err := a.HashContext(ctx)
				if err != nil {
					t.Fatal(err)
				}
				bHash, err := b.HashContext(ctx)
				if err != nil || aHash == bHash {
					t.Fatal("distinct comparison maps lost public hash discrimination", err)
				}
				if test.ConcreteComparisonMaps && (aHash != a.Hash() || bHash != b.Hash()) {
					t.Fatal("checked hash changed concrete hashes")
				}
			}
			checkPublic()
			for i, value := range []RuntimeValue{a, b, duplicate} {
				code, err := runtimeReferenceCodeContext(ctx, value)
				if err != nil || code != test.Hashes[i] {
					t.Fatal("raw source hash", i, code, test.Hashes[i], err)
				}
				if err := runtimeMarkReferenceHashContext(ctx, value); err != nil {
					t.Fatal(err)
				}
			}
			if got := observe(); got != test.After.Equality {
				t.Fatal("source equality after hashing", got, test.After.Equality)
			}
			checkPublic()
		})
	}
}
