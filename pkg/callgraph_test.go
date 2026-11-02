package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

func TestUpstreamCallGraphs(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/callgraph.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name   string
		Input  []Module
		Graphs []map[string][]string
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
			for i, m := range f.Input {
				graph := BuildCallGraph(m, f.Input, r.Table)
				got := map[string][]string{}
				for id, uses := range graph.All() {
					ids := []string{}
					for _, use := range uses {
						ids = append(ids, use.String())
					}
					sort.Strings(ids)
					got[id] = ids
				}
				if !reflect.DeepEqual(got, f.Graphs[i]) {
					t.Fatalf("%s: Go %v TS %v", m.Name, got, f.Graphs[i])
				}
			}
		})
	}
}

func TestCallGraphNestedDependencies(t *testing.T) {
	ms, err := ParseModules("module M { type T=int const n:T val x=def f(a:T):T=n; f(n) }")
	if err != nil {
		t.Fatal(err)
	}
	r, _ := ResolveLinkedModules(ms)
	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	before, _ := json.Marshal(ms)
	g := BuildCallGraph(ms[0], ms, r.Table)
	alias, constant, outer := ms[0].Declarations[0], ms[0].Declarations[1], ms[0].Declarations[2]
	local := outer.Expr.Opdef
	contains := func(id, use ID) bool {
		for _, v := range g.Get(id.String()) {
			if v == use {
				return true
			}
		}
		return false
	}
	for _, pair := range [][2]ID{{constant.ID, alias.ID}, {local.ID, alias.ID}, {local.ID, constant.ID}, {outer.ID, alias.ID}, {outer.ID, constant.ID}, {outer.ID, local.ID}} {
		if !contains(pair[0], pair[1]) {
			t.Fatalf("missing %v", pair)
		}
	}
	after, _ := json.Marshal(ms)
	if string(before) != string(after) {
		t.Fatal("input mutated")
	}
}
