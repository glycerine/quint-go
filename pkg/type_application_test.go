package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestTypeApplications(t *testing.T) {
	for _, tc := range []struct{ decls, annotation, want string }{
		{"type Pair[a,b]=(a,b)", "Pair[int,str]", "(int, str)"},
		{"type Box[a]=List[a] type Twice[a]=Box[Box[a]]", "Twice[int]", "List[List[int]]"},
		{"type Add[r]={x:int|r}", "Add[{y:str}]", "{ x: int, y: str }"},
		{"type Add[r]={x:int|r}", "Add[s]", "{ x: int | s }"},
		{"type Pair[a,b]=(a,b)", "Pair[b,a]", "(b, a)"},
	} {
		t.Run(tc.annotation, func(t *testing.T) {
			ms, err := ParseModules("module M { " + tc.decls + " const value:" + tc.annotation + " }")
			if err != nil {
				t.Fatal(err)
			}
			linked, _ := ResolveLinkedModules(ms)
			if len(linked.Errors) != 0 {
				t.Fatal(linked.Errors)
			}
			before, _ := json.Marshal(ms)
			r := NewTypeApplicationResolver(linked.Table)
			out, err := r.Resolve(*ms[0].Declarations[len(ms[0].Declarations)-1].TypeAnnotation)
			if err != nil {
				t.Fatal(err)
			}
			got := TypeString(SimplifyType(out))
			if got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
			for _, e := range r.Errors.All() {
				t.Fatal(e)
			}
			after, _ := json.Marshal(ms)
			if string(before) != string(after) {
				t.Fatal("input mutated")
			}
		})
	}
}
func TestTypeApplicationArity(t *testing.T) {
	ms, err := ParseModules("module M { type Pair[a,b]=(a,b) const x:Pair[int] const y:Pair[int,str,bool] }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	r := NewTypeApplicationResolver(linked.Table)
	for _, d := range ms[0].Declarations[1:] {
		out, err := r.Resolve(*d.TypeAnnotation)
		if err != nil || out.Kind != "app" {
			t.Fatalf("%v %+v", err, out)
		}
	}
	count := 0
	for _, e := range r.Errors.All() {
		if e.Message == nil {
			t.Fatal(e)
		}
		want := "too few arguments supplied: Pair only accepts 2 parameters"
		if count == 1 {
			want = "too many arguments supplied: Pair only accepts 2 parameters"
		}
		if *e.Message != want {
			t.Fatalf("%s", *e.Message)
		}
		count++
	}
	if count != 2 {
		t.Fatal(count)
	}
}
func TestTypeApplicationCycles(t *testing.T) {
	ms, err := ParseModules("module M { type Loop[a]=List[Loop[a]] const x:Loop[int] }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	_, err = NewTypeApplicationResolver(linked.Table).Resolve(*ms[0].Declarations[1].TypeAnnotation)
	if err == nil {
		t.Fatal("cycle accepted")
	}
}

func TestTypeApplicationInvalidReferences(t *testing.T) {
	ms, err := ParseModules("module M { type Abstract const x:Abstract[int] }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	resolver := NewTypeApplicationResolver(linked.Table)
	if _, err = resolver.Resolve(*ms[0].Declarations[1].TypeAnnotation); err == nil {
		t.Fatal("abstract application accepted")
	}
	typ, err := ParseType("Box[int]")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = resolver.Resolve(typ); err == nil {
		t.Fatal("missing IDs accepted")
	}
}

func TestResolveApplicationLookupTable(t *testing.T) {
	ms, err := ParseModules("module M { type Box[a]=List[a] const c:Box[int] def f(x:Box[int]):Box[int]=x val y=f(c) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	before, _ := json.Marshal(ms)
	resolver := NewTypeApplicationResolver(linked.Table)
	if err = resolver.ResolveLookupTable(); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolver.ResolveDeclarations(ms[0].Declarations)
	if err != nil {
		t.Fatal(err)
	}
	for _, def := range linked.Table.All() {
		if def.Parameter != nil && def.Parameter.TypeAnnotation != nil && def.Parameter.TypeAnnotation.Kind != "list" {
			t.Fatal("parameter table annotation")
		}
		if def.Declaration != nil && def.Declaration.Kind == "const" && def.Declaration.TypeAnnotation.Kind != "list" {
			t.Fatal("constant table annotation")
		}
	}
	if resolved[1].TypeAnnotation.Kind != "list" {
		t.Fatal("declaration annotation")
	}
	after, _ := json.Marshal(ms)
	if string(before) != string(after) {
		t.Fatal("lookup transformation mutated source IR")
	}
}

func TestUpstreamTypeApplications(t *testing.T) {
	var fixtures []struct {
		Name   string
		Input  []Module
		Output []Declaration
		Errors []struct {
			ID    string
			Error ErrorTree
		}
	}
	data, err := os.ReadFile("testdata/parser/type_application.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, _ := ResolveLinkedModules(f.Input)
			resolver := NewTypeApplicationResolver(linked.Table)
			if err = resolver.ResolveLookupTable(); err != nil {
				t.Fatal(err)
			}
			out, err := resolver.ResolveDeclarations(f.Input[0].Declarations)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(out)
			want, _ := json.Marshal(f.Output)
			if string(got) != string(want) {
				t.Fatalf("Go %s\nTS %s", got, want)
			}
			count := 0
			for id, e := range resolver.Errors.All() {
				if count >= len(f.Errors) || id != f.Errors[count].ID || !reflect.DeepEqual(e, f.Errors[count].Error) {
					t.Fatalf("error %s %+v", id, e)
				}
				count++
			}
			if count != len(f.Errors) {
				t.Fatal("error count")
			}
		})
	}
}
