package quint

import (
	"reflect"
	"testing"
)

func TestConstraintSolverConjunction(t *testing.T) {
	id := func(s string) ID { v, _ := ParseID(s); return v }
	typ := func(s string) Type {
		v, e := ParseType(s)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	eq := func(n, a, b string) Constraint {
		return Constraint{Kind: "eq", SourceID: id(n), Types: [2]Type{typ(a), typ(b)}}
	}
	u := TypeUnifier{}
	c := Constraint{Kind: "conjunction", Constraints: []Constraint{eq("1", "a", "int"), eq("2", "b", "List[a]")}}
	solved, err := u.SolveConstraint(c)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ApplySubstitution(solved.Substitutions, typ("b"))
	if err != nil || TypeString(out) != "List[int]" {
		t.Fatalf("%+v %v", solved, err)
	}
	// After the failure a is no longer substituted: a=str succeeds, but int=bool fails.
	c.Constraints = append(c.Constraints, eq("3", "int", "str"), eq("4", "a", "str"), eq("5", "int", "bool"))
	solved, err = u.SolveConstraint(c)
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	for k := range solved.Errors.All() {
		keys = append(keys, k)
	}
	if !reflect.DeepEqual(keys, []string{"3", "5"}) || solved.Substitutions != nil {
		t.Fatalf("%v %+v", keys, solved.Substitutions)
	}
}
func TestConstraintDefinedAndOrdering(t *testing.T) {
	ms, err := ParseModules("module M { type Box[a]=List[a] const x:Box[int] }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	u := TypeUnifier{Table: linked.Table}
	target, _ := ParseType("List[int]")
	source, _ := ParseID("99")
	c := Constraint{Kind: "isDefined", SourceID: source, Type: target}
	s, err := u.SolveConstraint(c)
	if err != nil || len(s.Substitutions) != 1 {
		t.Fatalf("%+v %v", s, err)
	}
	c.Type = Type{Kind: "bool"}
	s, err = u.SolveConstraint(c)
	if err != nil {
		t.Fatal(err)
	}
	if e := s.Errors.Get("99"); e.Message == nil || *e.Message != "Expected type is not defined" {
		t.Fatal(e)
	}
	c = Constraint{Kind: "conjunction", Constraints: []Constraint{c, {Kind: "eq", SourceID: source, Types: [2]Type{{Kind: "int"}, {Kind: "str"}}}}}
	s, err = u.SolveConstraint(c)
	if err != nil {
		t.Fatal(err)
	}
	if *s.Errors.Get("99").Message != "Expected type is not defined" {
		t.Fatal("kind ordering or duplicate overwrite")
	}
	if c.Constraints[0].Kind != "isDefined" {
		t.Fatal("input reordered")
	}
	if _, err = u.SolveConstraint(Constraint{Kind: "bad"}); err == nil {
		t.Fatal("invalid kind accepted")
	}
}
