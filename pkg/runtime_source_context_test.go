package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

type runtimeSourceMemoSnapshot struct {
	ID   ID
	Memo []ID
}

type runtimeSourceContextFixture struct {
	Name, Source, Module string
	Steps                []struct {
		Name         string
		Registration []runtimeVariableRegistration
		Names        []string
		RootMemo     []ID
		Instances    []runtimeSourceMemoSnapshot
	}
}

func loadRuntimeSourceContexts(t *testing.T) []runtimeSourceContextFixture {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/runtime_instance_context.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []runtimeSourceContextFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func TestUpstreamRuntimeSourceContexts(t *testing.T) {
	t.Parallel()
	for _, fixture := range loadRuntimeSourceContexts(t) {
		t.Run(fixture.Name, func(t *testing.T) {
			modules, err := ParseModules(fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			graph, err := newRuntimeSourceGraph(modules)
			if err != nil {
				t.Fatal(err)
			}
			preparation := newRuntimeSourcePreparation(&runtimeSourceModule{graph: graph, name: fixture.Module})
			storage := NewVariableStorage()
			for index, step := range fixture.Steps {
				handled, err := preparation.PrepareDefinition(context.Background(), step.Name, storage)
				if !handled || err != nil {
					t.Fatal(index, handled, err)
				}
				assertRuntimeVariables(t, storage, runtimeVariableSnapshot{Registration: step.Registration, Names: step.Names, Picks: []nondetPickSnapshot{}})
				root := []ID{}
				instances := []runtimeSourceMemoSnapshot{}
				for id, memo := range preparation.memos.All() {
					keys := []ID{}
					for key := range memo.All() {
						keys = append(keys, key)
					}
					if id == (ID{}) {
						root = keys
					} else {
						instances = append(instances, runtimeSourceMemoSnapshot{ID: id, Memo: keys})
					}
				}
				if !reflect.DeepEqual(root, step.RootMemo) || !reflect.DeepEqual(instances, step.Instances) {
					t.Fatalf("step %d %s: root memo %v want %v; instances %v want %v", index, step.Name, root, step.RootMemo, instances, step.Instances)
				}
				for _, register := range storage.current.All() {
					if register.Value.Valid() {
						t.Fatal("source preparation evaluated variable effects")
					}
				}
			}
		})
	}
}

func TestRuntimeSourceContextCancellationOwnershipAndUpdate(t *testing.T) {
	t.Parallel()
	modules, err := ParseModules("module M {var x:int val result=x}")
	if err != nil {
		t.Fatal(err)
	}
	graph, err := newRuntimeSourceGraph(modules)
	if err != nil {
		t.Fatal(err)
	}
	// The owned source graph must retain both the selected definition and its
	// original resolved reference when callers later edit their input IR.
	modules[0].Declarations[0].Name = "changed"
	modules[0].Declarations[1].Expr.Name = "changed"
	preparation := newRuntimeSourcePreparation(&runtimeSourceModule{graph: graph, name: "M"})
	storage := NewVariableStorage()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := preparation.PrepareDefinition(ctx, "result", storage); !errors.Is(err, context.Canceled) || storage.current.Len() != 0 {
		t.Fatal(err, storage.VarNames())
	}
	if handled, err := preparation.PrepareDefinition(context.Background(), "result", storage); !handled || err != nil {
		t.Fatal(handled, err)
	}
	if !reflect.DeepEqual(storage.VarNames(), []string{"x"}) {
		t.Fatal(storage.VarNames())
	}
	var identity runtimeSourceIdentity
	var binding RuntimeVariableBinding
	for key, value := range preparation.bindings.All() {
		identity, binding = key, value
	}
	preparation.UpdateSource(nil)
	if handled, err := preparation.PrepareDefinition(context.Background(), "result", storage); handled || err != nil {
		t.Fatal(handled, err)
	}
	if got, exists := preparation.Binding(identity); !exists || got != binding {
		t.Fatal("source update discarded a compiled binding", got, exists)
	}
	if handled, err := newRuntimeSourcePreparation(&runtimeSourceModule{graph: graph, name: "missing"}).PrepareDefinition(context.Background(), "result", storage); handled || err != nil {
		t.Fatal(handled, err)
	}
	var zero *runtimeSourcePreparation
	if _, exists := zero.Binding(identity); exists {
		t.Fatal("nil preparation exposed a binding")
	}
	cyclic, err := ParseModules("module M {val a=b val b=a}")
	if err != nil {
		t.Fatal(err)
	}
	cyclicGraph, err := newRuntimeSourceGraph(cyclic)
	if err != nil {
		t.Fatal(err)
	}
	cycle := newRuntimeSourcePreparation(&runtimeSourceModule{graph: cyclicGraph, name: "M"})
	_, err = cycle.PrepareDefinition(context.Background(), "a", NewVariableStorage())
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != "QNT501" || diagnostic.Message != "Cyclic runtime source compilation" {
		t.Fatal("source cycle was not bounded", err)
	}
}

func TestRuntimeSourceProvenanceTransportResetsExistingModule(t *testing.T) {
	t.Parallel()
	ids := NewIDGenerator()
	modules, err := parseModulesWithIDs("module A {var x:int action init=x'=0} module M {import A as I action start=I::init}", ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := ResolveLinkedModules(modules)
	if err != nil || len(linked.Errors) != 0 {
		t.Fatal(err, linked.Errors)
	}
	flat, err := FlattenModules(modules, linked.Table, ids, nil, AnalysisOutput{})
	if err != nil {
		t.Fatal(err)
	}
	module := flat.Modules[len(flat.Modules)-1]
	if module.runtimeSource == nil {
		t.Fatal("source module lost its original compilation graph")
	}
	data, err := json.Marshal(module)
	if err != nil {
		t.Fatal(err)
	}
	// Reusing a destination must clear private fields just like decoding into
	// a new module, otherwise a transported flat IR can retain source behavior.
	if err := json.Unmarshal(data, &module); err != nil {
		t.Fatal(err)
	}
	if module.runtimeSource != nil {
		t.Fatal("transport retained source compilation graph")
	}
	var inspect func(*Expr)
	inspect = func(expression *Expr) {
		if expression == nil {
			return
		}
		if expression.runtimeSource != nil {
			t.Fatal("transport retained source expression identity")
		}
		for i := range expression.Args {
			inspect(&expression.Args[i])
		}
		inspect(expression.Body)
		if expression.Opdef != nil {
			inspect(expression.Opdef.Expr)
		}
	}
	for _, declaration := range module.Declarations {
		if declaration.runtimeVariable != nil {
			t.Fatal("transport retained source variable origin")
		}
		inspect(declaration.Expr)
	}
}
