package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"sync"
	"testing"
)

func builtinTypeIDPointers(t *testing.T, typ *Type) []*ID {
	t.Helper()
	var ids []*ID
	if err := WalkType(typ, func(node *Type) (bool, error) {
		if node.ID != nil {
			ids = append(ids, node.ID)
		}
		return true, nil
	}); err != nil {
		t.Fatal(err)
	}
	return ids
}

func TestFixedBuiltinTypeProvenanceOwnership(t *testing.T) {
	t.Parallel()
	replacement, err := ParseID("9007199254740993123456789")
	if err != nil {
		t.Fatal(err)
	}
	for name, template := range fixedBuiltinTypeCache {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			first, err := BuiltinTypeSignature(name, 0)
			if err != nil {
				t.Fatal(err)
			}
			second, err := BuiltinTypeSignature(name, 42)
			if err != nil {
				t.Fatal(err)
			}
			firstIDs := builtinTypeIDPointers(t, &first.Type)
			secondIDs := builtinTypeIDPointers(t, &second.Type)
			templateIDs := builtinTypeIDPointers(t, &template.Type)
			if len(firstIDs) == 0 || len(firstIDs) != len(secondIDs) || len(firstIDs) != len(templateIDs) {
				t.Fatal("provenance ID inventory changed")
			}
			seen := map[*ID]bool{}
			for i, id := range firstIDs {
				if seen[id] || id == secondIDs[i] || id == templateIDs[i] || secondIDs[i] == templateIDs[i] {
					t.Fatal("type provenance pointers are shared")
				}
				seen[id] = true
				*id = replacement
			}
			third, err := BuiltinTypeSignature(name, 999)
			if err != nil || !reflect.DeepEqual(second, third) || !reflect.DeepEqual(template, third) {
				t.Fatal("mutating provenance changed another signature", err)
			}
		})
	}
}

func TestFixedBuiltinTypeProvenanceConcurrentOwnership(t *testing.T) {
	t.Parallel()
	var workers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for name, template := range fixedBuiltinTypeCache {
				scheme, err := BuiltinTypeSignature(name, 3)
				if err != nil || !reflect.DeepEqual(scheme, template) {
					t.Errorf("%s: initial provenance changed: %v", name, err)
					return
				}
				if err := WalkType(&scheme.Type, func(node *Type) (bool, error) {
					if node.ID != nil {
						*node.ID = ID{}
					}
					return true, nil
				}); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	workers.Wait()
}

func TestBuiltinTypeCloneConstructorProvenanceOwnership(t *testing.T) {
	t.Parallel()
	typ, _, err := ParseTypeWithSource("types", "Result[List[int], {value:bool}]")
	if err != nil {
		t.Fatal(err)
	}
	original := TypeScheme{Type: typ, TypeVariables: []string{}, RowVariables: []string{}}
	before, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	clone, err := cloneBuiltinTypeScheme(original)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(clone, original) || clone.Type.Ctor.ID == original.Type.Ctor.ID {
		t.Fatal("constructor provenance was changed or shared")
	}
	for _, id := range builtinTypeIDPointers(t, &clone.Type) {
		*id = ID{}
	}
	after, err := json.Marshal(original)
	if err != nil || string(before) != string(after) {
		t.Fatal("clone shares type-node provenance with its input", err)
	}
}

func TestBuiltinTypeProvenanceDoesNotChangePlainTypeParser(t *testing.T) {
	t.Parallel()
	// ParseType intentionally omits source identities. BuiltinTypeSignature
	// supplies the reference signature parser's separate local provenance.
	parsed, err := parseBuiltinTypeScheme(fixedBuiltinTypes["iadd"])
	if err != nil {
		t.Fatal(err)
	}
	assertBuiltinTypeTemplateNoIDs(t, parsed.Type)
	builtin, err := BuiltinTypeSignature("iadd", 2)
	if err != nil || builtin.Type.ID == nil || builtin.Type.Res.ID == nil {
		t.Fatal("builtin parser provenance missing", err)
	}
}

func TestBuiltinTypeProvenanceThroughInference(t *testing.T) {
	t.Parallel()
	type entry struct {
		ID     string
		Scheme TypeScheme
	}
	var fixtures []struct {
		Name  string
		Input []Module
		Types []entry
	}
	// This saved reference inferrer output already retains every type-node ID.
	// Keep the earlier normalized comparison and additionally check provenance.
	data, err := os.ReadFile("testdata/parser/inference.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			linked, _ := ResolveLinkedModules(fixture.Input)
			inferrer := NewTypeInferrer(linked.Table, nil)
			if _, err := inferrer.InferTypes(fixture.Input[0].Declarations); err != nil {
				t.Fatal(err)
			}
			actual := []entry{}
			for id, scheme := range inferrer.Context.Types.All() {
				actual = append(actual, entry{id, scheme})
			}
			if !reflect.DeepEqual(actual, fixture.Types) {
				got, _ := json.Marshal(actual)
				want, _ := json.Marshal(fixture.Types)
				t.Fatalf("raw type inference provenance:\n got %s\nwant %s", got, want)
			}
		})
	}
}
