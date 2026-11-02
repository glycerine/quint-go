package quint

import "testing"

func TestTypeUnification(t *testing.T) {
	u := TypeUnifier{}
	parse := func(s string) Type {
		t.Helper()
		v, e := ParseType(s)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	for _, pair := range [][2]string{
		{"int", "int"}, {"a", "List[int]"}, {"Set[a]", "Set[str]"},
		{"a -> a", "int -> int"}, {"(a,b)=>a", "(int,str)=>int"},
		{"{x:a|r}", "{x:int,y:str}"}, {"{x:int|r}", "{y:str|s}"},
		{"(a,b)", "(int,str)"},
	} {
		a, b := parse(pair[0]), parse(pair[1])
		subs, e := u.Unify(a, b)
		if e != nil {
			t.Fatalf("%v: %s", pair, e.String())
		}
		x, err := ApplySubstitution(subs, a)
		if err != nil {
			t.Fatal(err)
		}
		y, err := ApplySubstitution(subs, b)
		if err != nil {
			t.Fatal(err)
		}
		// Row field order is semantic, not textual; re-unification must be empty.
		again, e := u.Unify(SimplifyType(x), SimplifyType(y))
		if e != nil || len(again) != 0 {
			t.Fatalf("%v: %v %+v", pair, e, again)
		}
	}
	for _, pair := range [][2]string{
		{"int", "str"}, {"a", "List[a]"}, {"a -> a", "int -> str"},
		{"(int)=>int", "(int,str)=>int"}, {"{x:int}", "{y:int}"},
		{"{x:int|r}", "{y:int|r}"}, {"{x:int}", "{x:str}"},
	} {
		if _, e := u.Unify(parse(pair[0]), parse(pair[1])); e == nil {
			t.Fatalf("accepted %v", pair)
		}
	}
}

func TestUnifyAliasesAndComposition(t *testing.T) {
	ms, err := ParseModules("module M { type A=int type Opaque const a:A const b:Opaque }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	u := TypeUnifier{Table: linked.Table}
	if _, e := u.Unify(*ms[0].Declarations[2].TypeAnnotation, Type{Kind: "int"}); e != nil {
		t.Fatal(e)
	}
	if _, e := u.Unify(*ms[0].Declarations[3].TypeAnnotation, Type{Kind: "int"}); e == nil {
		t.Fatal("abstract mismatch")
	}
	s, e := u.Compose([]Substitution{{Kind: "type", Name: "a", Type: Type{Kind: "int"}}}, []Substitution{{Kind: "type", Name: "b", Type: Type{Kind: "var", Name: "a"}}})
	if e != nil || len(s) != 2 || s[0].Name != "b" || s[0].Type.Kind != "int" {
		t.Fatalf("%+v %v", s, e)
	}
	if _, e = u.Compose([]Substitution{{Kind: "type", Name: "a", Type: Type{Kind: "int"}}}, []Substitution{{Kind: "type", Name: "a", Type: Type{Kind: "str"}}}); e == nil {
		t.Fatal("conflict accepted")
	}
}
