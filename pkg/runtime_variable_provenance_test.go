package quint

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestRuntimeVariableProvenanceThroughFlattening(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source, suffix, display string
	}{
		{"local", "module M {var x:int action start=x'=1}", "", "x"},
		{"ordinary_alias", "module A {var x:int action init=x'=1} module M {import A as I action start=I::init}", "", "x"},
		{"shared_aliases", "module A {var x:int action init=x'=1} module M {import A as I import A as J action start=I::init and J::init}", "", "x"},
		{"instance", "module A {const C:int var x:int action init=x'=C} module M {import A(C=1) as I action start=I::init}", "#I#M", "M::I::x"},
		{"nested_instance", "module A {const N:int var x:int action update=x'=N} module B {const C:int import A(N=C) as Inner action init=Inner::update} module M {import B(C=1) as Outer action start=Outer::init}", "#Inner#B", "B::Inner::x"},
		{"ordinary_inside_instance", "module A {var x:int} module B {const C:int import A as Shared action init=Shared::x'=C} module M {import B(C=1) as Outer action start=Outer::init}", "#Outer#M", "M::Outer::x"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			ids := NewIDGenerator()
			modules, err := parseModulesWithIDs(test.source, ids)
			if err != nil {
				t.Fatal(err)
			}
			var sourceVariable Declaration
			for _, module := range modules {
				for _, declaration := range module.Declarations {
					if declaration.Kind == "var" {
						sourceVariable = declaration
					}
				}
			}
			linked, err := ResolveLinkedModules(modules)
			if err != nil || len(linked.Errors) > 0 {
				t.Fatal(err, linked.Errors)
			}
			flat, err := FlattenModules(modules, linked.Table, ids, nil, AnalysisOutput{})
			if err != nil {
				t.Fatal(err)
			}
			var main Module
			for _, module := range flat.Modules {
				if module.Name == "M" {
					main = module
				}
			}
			evaluator, err := NewModuleEvaluator(main, nil, false)
			if err != nil {
				t.Fatal(err)
			}
			if err := evaluator.PrepareDefinition(context.Background(), "start"); err != nil {
				t.Fatal(err)
			}
			want := []runtimeVariableRegistration{{Key: sourceVariable.ID.String() + test.suffix, Name: test.display}}
			got := []runtimeVariableRegistration{}
			for key, register := range evaluator.Storage.current.All() {
				got = append(got, runtimeVariableRegistration{Key: key, Name: register.Name})
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatal("registered source cells", got, want)
			}
			// Private annotations must not mutate caller IR or its lookup table.
			for _, module := range modules {
				for _, declaration := range module.Declarations {
					if declaration.runtimeVariable != nil {
						t.Fatal("input declaration acquired runtime provenance")
					}
				}
			}
			for _, definition := range linked.Table.All() {
				if definition.Declaration != nil && definition.Declaration.runtimeVariable != nil {
					t.Fatal("input lookup acquired runtime provenance")
				}
			}
			// The reference IR carries no variable-origin wire fields. Source
			// evaluation retains them in memory; transporting a flattened IR
			// intentionally returns the declaration-ID/name fallback behavior.
			data, err := json.Marshal(main)
			if err != nil {
				t.Fatal(err)
			}
			var transported Module
			if err := json.Unmarshal(data, &transported); err != nil {
				t.Fatal(err)
			}
			for _, declaration := range transported.Declarations {
				if declaration.runtimeVariable != nil {
					t.Fatal("transported private provenance")
				}
			}
			again, err := json.Marshal(transported)
			if err != nil || string(data) != string(again) {
				t.Fatal("private provenance changed transport encoding", err)
			}
		})
	}
}
