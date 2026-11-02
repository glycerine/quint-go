package quint

import (
	"encoding/json"
	"github.com/glycerine/insdict"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestUpstreamDeclarationSort(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/declaration_sort.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name    string
		Input   []Module
		Results []struct{ Sorted, Cycles []string }
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			r, err := ResolveLinkedModules(f.Input)
			if err != nil {
				t.Fatal(err)
			}
			output, diagnostics := SortModuleDeclarations(f.Input, r.Table)
			expectedErrors := 0
			for i, m := range f.Input {
				sorted, cycles := SortDeclarations(BuildCallGraph(m, f.Input, r.Table), m.Declarations)
				ids, residual := []string{}, []string{}
				for _, d := range sorted {
					ids = append(ids, d.ID.String())
				}
				for _, id := range cycles {
					residual = append(residual, id.String())
				}
				sort.Strings(residual)
				if !reflect.DeepEqual(ids, f.Results[i].Sorted) || !reflect.DeepEqual(residual, f.Results[i].Cycles) {
					t.Fatalf("Go %v %v TS %+v", ids, residual, f.Results[i])
				}
				if !reflect.DeepEqual(output[i].Declarations, sorted) {
					t.Fatal("phase output")
				}
				expectedErrors += len(cycles)
			}
			if len(diagnostics) != expectedErrors {
				t.Fatal("cycle diagnostic count")
			}
			for _, d := range diagnostics {
				if d.Code != QNT099 || d.Reference == nil || d.Message != "Found cyclic declarations. Use fold and foldl instead of recursion" {
					t.Fatal(d)
				}
			}
		})
	}
}

func TestDeclarationSortLayersAndResiduals(t *testing.T) {
	id := func(s string) ID {
		v, e := ParseID(s)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	g := insdict.NewDict[string, []ID]()
	g.Set("3", []ID{id("2")})
	g.Set("4", []ID{id("99")})
	g.Set("5", []ID{id("6")})
	g.Set("6", []ID{id("5")})
	g.Set("7", []ID{id("6")})
	ds := []Declaration{}
	for _, n := range []string{"2", "10", "3", "4", "5", "6", "7"} {
		ds = append(ds, Declaration{ID: id(n), Kind: "const", Name: "n"})
	}
	before := append([]ID(nil), g.Get("3")...)
	sorted, cycles := SortDeclarations(g, ds)
	got := []string{}
	for _, d := range sorted {
		got = append(got, d.ID.String())
	}
	if !reflect.DeepEqual(got, []string{"10", "2", "3", "4"}) {
		t.Fatal(got)
	}
	residual := []string{}
	for _, v := range cycles {
		residual = append(residual, v.String())
	}
	if !reflect.DeepEqual(residual, []string{"5", "6", "7"}) {
		t.Fatal(residual)
	}
	if !reflect.DeepEqual(before, g.Get("3")) {
		t.Fatal("graph mutated")
	}
}

func TestSortNestedRecursion(t *testing.T) {
	ms, err := ParseModules("module M { val good=1 val bad=def f(x)=f(x); f(1) val blocked=bad }")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := ResolveLinkedModules(ms)
	sorted, errors := SortModuleDeclarations(ms, r.Table)
	if len(sorted[0].Declarations) != 1 || sorted[0].Declarations[0].Name != "good" || len(errors) != 3 {
		t.Fatalf("%+v %+v", sorted, errors)
	}
	if len(ms[0].Declarations) != 3 {
		t.Fatal("input mutated")
	}
}
