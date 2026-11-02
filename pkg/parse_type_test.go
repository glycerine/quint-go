package quint

import "testing"

func TestParseType(t *testing.T) {
	for _, c := range [][2]string{{"int", "int"}, {"bool", "bool"}, {"str", "str"}, {"a", "a"}, {"M::T", "M::T"}, {"Set[List[int]]", "Set[List[int]]"}, {"int -> bool -> str", "(int -> (bool -> str))"}, {"(int, bool) => str", "(int, bool) => str"}, {"() => int", "() => int"}, {"(int)", "int"}, {"()", "()"}, {"(int, str)", "(int, str)"}, {"{ x: int, y: a | r }", "{ x: int, y: a | r }"}, {"{}", "{}"}, {"Box[int, Set[a]]", "Box[int, Set[a]]"}} {
		typ, err := ParseType(c[0])
		if err != nil {
			t.Fatalf("%s: %v", c[0], err)
		}
		if err = typ.Validate(); err != nil {
			t.Fatal(err)
		}
		if got := TypeString(typ); got != c[1] {
			t.Fatalf("%s: got %q want %q", c[0], got, c[1])
		}
	}
	tuple, err := ParseType("(int, bool)")
	if err != nil {
		t.Fatal(err)
	}
	if tuple.Fields.Fields[0].Name != "0" || tuple.Fields.Fields[1].Name != "1" {
		t.Fatal("tuple indices")
	}
	for _, s := range []string{"", "Set[", "int extra", "Foo(int)", "{ a::b: int }", "int ->", "@"} {
		if _, err := ParseType(s); err == nil {
			t.Errorf("accepted %q", s)
		}
	}
}
