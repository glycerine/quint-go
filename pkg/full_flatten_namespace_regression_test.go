package quint

import (
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"
)

func TestFullFlattenImportedLocalNamespaceRegression(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/full_flatten_alias_pending.json")
	if err != nil {
		t.Fatal(err)
	}
	var witnesses []struct {
		flattenAliasFixture
		AnalysisMode string
		Error        string
	}
	if err := json.Unmarshal(data, &witnesses); err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, witness := range witnesses {
		if witness.AnalysisMode == "" {
			continue
		}
		checked++
		t.Run(witness.AnalysisMode, func(t *testing.T) {
			// Upstream's retained namespace metadata prefixes Shared twice
			// across an instance boundary. Preserve Go's existing support for
			// this valid program while retaining correct lookup metadata.
			if !strings.Contains(witness.Error, "Name 'M::Outer::Shared::x' not found") {
				t.Fatalf("unexpected reference boundary: %s", witness.Error)
			}
			linked, err := ResolveLinkedModules(witness.InputBeforeAnalysis)
			if err != nil || len(linked.Errors) != 0 {
				t.Fatal(err, linked.Errors)
			}
			ids := NewIDGenerator()
			if _, ok := ids.next.SetString(witness.LastID, 10); !ok {
				t.Fatal("invalid saved ID state")
			}
			ids.next.Add(&ids.next, big.NewInt(1))
			result, err := FlattenModules(witness.Input, linked.Table, ids, witness.sourceMap(t, witness.InputSources), witness.InputAnalysis.output())
			if err != nil {
				t.Fatal(err)
			}
			foundModules := 0
			for _, module := range result.Modules {
				if module.Name != "M::Outer" && module.Name != "M" {
					continue
				}
				foundModules++
				variables := []string{}
				var variable Declaration
				var initialize Declaration
				for _, declaration := range module.Declarations {
					if declaration.Kind == "var" {
						variables = append(variables, declaration.Name)
						variable = declaration
					}
					if declaration.Name == "M::Outer::init" {
						initialize = declaration
					}
				}
				flattenAliasEqual(t, "concrete variable namespace in "+module.Name, variables, []string{"M::Outer::Shared::x"})
				if initialize.Expr == nil || len(initialize.Expr.Args) != 2 {
					t.Fatalf("missing instance initializer in %s", module.Name)
				}
				assigned := initialize.Expr.Args[0]
				if assigned.Kind != "name" || assigned.Name != variable.Name {
					t.Fatalf("initializer targets wrong namespace: %+v", assigned)
				}
				binding, ok := result.Table.Get2(assigned.ID)
				if !ok || binding.ID != variable.ID || !binding.Hidden {
					t.Fatalf("initializer variable binding: %+v", binding)
				}
				flattenAliasEqual(t, "retained prior import namespace", binding.Namespaces, []string{"Shared"})
			}
			if foundModules != 2 {
				t.Fatal("instance or importing module was omitted")
			}
			relinked, err := ResolveLinkedModules(result.Modules)
			if err != nil || len(relinked.Errors) != 0 {
				t.Fatal(err, relinked.Errors)
			}
		})
	}
	if checked != 2 {
		t.Fatalf("expected empty and analyzed reference witnesses, got %d", checked)
	}
}
