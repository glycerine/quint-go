package quint

import (
	"encoding/json"
	"github.com/glycerine/insdict"
	"testing"
)

func TestInlineProjectAliases(t *testing.T) {
	modules, err := ParseModules("module M { type Opaque type T=int const c:List[T] def f(x:T):T=val local:T=x; local }")
	if err != nil {
		t.Fatal(err)
	}
	linked, err := ResolveLinkedModules(modules)
	if err != nil || len(linked.Errors) > 0 {
		t.Fatal(err, linked.Errors)
	}
	before, _ := json.Marshal(modules)
	types := insdict.NewDict[string, TypeScheme]()
	types.Set("first", TypeScheme{Type: *modules[0].Declarations[2].TypeAnnotation, TypeVariables: []string{"a"}})
	effects := insdict.NewDict[string, EffectScheme]()
	modes := insdict.NewDict[string, Qualifier]()
	result, err := InlineTypeAliases(modules, linked.Table, AnalysisOutput{Types: types, Effects: effects, Modes: modes})
	if err != nil {
		t.Fatal(err)
	}
	if result.Analysis.Effects != effects || result.Analysis.Modes != modes {
		t.Fatal("effect/mode tables replaced")
	}
	if result.Analysis.Types == types || result.Analysis.Types.Get("first").Type.Elem.Kind != "int" {
		t.Fatal("analysis alias not expanded")
	}
	if result.Analysis.Types.Get("first").TypeVariables[0] != "a" {
		t.Fatal("quantification lost")
	}
	if result.Modules[0].Declarations[0].Type != nil {
		t.Fatal("opaque type changed")
	}
	for _, d := range result.Modules[0].Declarations {
		_, err := TransformDeclarationTypes(d, func(typ Type) (Type, error) {
			err := WalkType(&typ, func(n *Type) (bool, error) {
				if n.Kind == "const" && n.Name == "T" {
					t.Fatal("annotation alias retained")
				}
				return true, nil
			})
			return typ, err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	count := 0
	oldKeys, newKeys := []string{}, []string{}
	for id, def := range result.Table.All() {
		newKeys = append(newKeys, id.String())
		if def.Parameter != nil && def.Parameter.TypeAnnotation != nil {
			count++
			if def.Parameter.TypeAnnotation.Kind != "int" {
				t.Fatal("parameter alias retained")
			}
			def.Parameter.TypeAnnotation.Kind = "str"
			if linked.Table.Get(id).Parameter.TypeAnnotation.Kind != "const" {
				t.Fatal("parameter input mutated")
			}
		}
	}
	for id := range linked.Table.All() {
		oldKeys = append(oldKeys, id.String())
	}
	a, _ := json.Marshal(oldKeys)
	b, _ := json.Marshal(newKeys)
	if string(a) != string(b) || count == 0 {
		t.Fatal("lookup insertion order/parameters lost")
	}
	after, _ := json.Marshal(modules)
	if string(before) != string(after) || types.Get("first").Type.Elem.Kind != "const" {
		t.Fatal("input mutated")
	}
}

func TestInlineProjectAliasCycle(t *testing.T) {
	modules, err := ParseModules("module M { type T=List[T] const c:T }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(modules)
	if _, err := InlineTypeAliases(modules, linked.Table, AnalysisOutput{}); err == nil {
		t.Fatal("cyclic alias accepted")
	}
}
