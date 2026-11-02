package quint

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sync"
	"testing"
)

type dynamicBuiltinTypeIDFixture struct {
	Name   string
	Arity  int
	Scheme TypeScheme
}

func readDynamicBuiltinTypeIDFixtures(t *testing.T) []dynamicBuiltinTypeIDFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/builtin_types_dynamic_ids.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []dynamicBuiltinTypeIDFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 52 {
		t.Fatalf("raw dynamic signature inventory: got %d, want 52", len(fixtures))
	}
	return fixtures
}

func TestUpstreamDynamicBuiltinTypeIDs(t *testing.T) {
	t.Parallel()
	seen := map[string]bool{}
	for _, fixture := range readDynamicBuiltinTypeIDFixtures(t) {
		name := fmt.Sprintf("%s/%d", fixture.Name, fixture.Arity)
		if seen[name] {
			t.Fatalf("duplicate raw dynamic signature %s", name)
		}
		seen[name] = true
		switch fixture.Arity {
		case 0, 1, 2, 3, 9, 33:
		default:
			t.Fatalf("unexpected raw dynamic signature arity: %s", name)
		}
		t.Run(name, func(t *testing.T) {
			got, err := BuiltinTypeSignature(fixture.Name, fixture.Arity)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, fixture.Scheme) {
				actual, _ := json.Marshal(got)
				expected, _ := json.Marshal(fixture.Scheme)
				t.Fatalf("raw reference mismatch:\n got %s\nwant %s", actual, expected)
			}
		})
	}
}

func TestDynamicBuiltinTypeIDOwnership(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"List", "Set", "Map", "and", "or", "actionAll", "actionAny", "matchVariant", "tuples"} {
		original, err := BuiltinTypeSignature(name, 5)
		if err != nil {
			t.Fatal(err)
		}
		independent, err := BuiltinTypeSignature(name, 5)
		if err != nil {
			t.Fatal(err)
		}
		before, err := json.Marshal(independent)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[*ID]bool{}
		for _, scheme := range []*TypeScheme{&original, &independent} {
			if err := WalkType(&scheme.Type, func(typ *Type) (bool, error) {
				if typ.ID == nil {
					t.Fatalf("%s: %s node has no ID", name, typ.Kind)
				}
				if seen[typ.ID] {
					t.Fatalf("%s: signature nodes share mutable ID pointers", name)
				}
				seen[typ.ID] = true
				return true, nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		if err := WalkType(&original.Type, func(typ *Type) (bool, error) { *typ.ID = ID{}; return true, nil }); err != nil {
			t.Fatal(err)
		}
		after, err := json.Marshal(independent)
		if err != nil || string(before) != string(after) {
			t.Fatalf("%s: mutating another signature's IDs changed the tree: %v", name, err)
		}
		fresh, err := BuiltinTypeSignature(name, 5)
		if err != nil || !reflect.DeepEqual(fresh, independent) {
			t.Fatalf("%s: subsequent call retained ID mutation: %v", name, err)
		}
	}
}

func TestDynamicBuiltinTypeIDsConcurrent(t *testing.T) {
	t.Parallel()
	fixtures := readDynamicBuiltinTypeIDFixtures(t)
	var workers sync.WaitGroup
	for range 8 {
		workers.Go(func() {
			for _, fixture := range fixtures {
				got, err := BuiltinTypeSignature(fixture.Name, fixture.Arity)
				if err != nil || !reflect.DeepEqual(got, fixture.Scheme) {
					t.Errorf("concurrent %s/%d differs: %v", fixture.Name, fixture.Arity, err)
					return
				}
				if err := WalkType(&got.Type, func(typ *Type) (bool, error) { *typ.ID = ID{}; return true, nil }); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	workers.Wait()
}
