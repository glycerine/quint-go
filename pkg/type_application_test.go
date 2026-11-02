package quint

import (
	"encoding/json"
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
