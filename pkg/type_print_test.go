package quint

import "testing"

func TestTypePrinting(t *testing.T) {
	integer := Type{Kind: "int"}
	boolean := Type{Kind: "bool"}
	empty := Row{Kind: "empty"}
	cases := []struct {
		typ  Type
		want string
	}{
		{integer, "int"}, {boolean, "bool"}, {Type{Kind: "str"}, "str"}, {Type{Kind: "var", Name: "a"}, "a"}, {Type{Kind: "const", Name: "T"}, "T"},
		{Type{Kind: "set", Elem: &integer}, "Set[int]"}, {Type{Kind: "list", Elem: &boolean}, "List[bool]"},
		{Type{Kind: "fun", Arg: &integer, Res: &boolean}, "(int -> bool)"},
		{Type{Kind: "oper", Args: []Type{integer, boolean}, Res: &integer}, "(int, bool) => int"},
		{Type{Kind: "app", Ctor: &Type{Kind: "const", Name: "Box"}, Args: []Type{integer}}, "Box[int]"},
		{UnitType(), "()"},
		{Type{Kind: "rec", Fields: &empty}, "{}"},
		{Type{Kind: "rec", Fields: &Row{Kind: "row", Fields: []RowField{{Name: "x", Type: integer}}, Other: &Row{Kind: "var", Name: "r"}}}, "{ x: int | r }"},
		{Type{Kind: "sum", Fields: &Row{Kind: "row", Fields: []RowField{{Name: "None", Type: UnitType()}}, Other: &Row{Kind: "row", Fields: []RowField{{Name: "Some", Type: integer}}, Other: &Row{Kind: "var", Name: "hidden"}}}}, "(None | Some(int))"},
	}
	for _, c := range cases {
		if got := TypeString(c.typ); got != c.want {
			t.Errorf("%s: got %q want %q", c.typ.Kind, got, c.want)
		}
	}
	row := Row{Kind: "row", Fields: []RowField{{Name: "x", Type: integer}}, Other: &Row{Kind: "row", Fields: []RowField{{Name: "y", Type: boolean}}, Other: &empty}}
	if got := RowString(row); got != "{ x: int, y: bool }" {
		t.Fatal(got)
	}
	for _, q := range []Qualifier{PureVal, PureDef, Val, Def, Action, Run, Temporal, Nondet} {
		want := string(q)
		if q == PureVal {
			want = "pure val"
		}
		if q == PureDef {
			want = "pure def"
		}
		if QualifierString(q) != want {
			t.Fatal(q)
		}
	}
}
func TestConstraintPrinting(t *testing.T) {
	c := Constraint{Kind: "conjunction", Constraints: []Constraint{{Kind: "empty"}, {Kind: "eq", Types: [2]Type{{Kind: "int"}, {Kind: "var", Name: "a"}}}, {Kind: "isDefined", Type: Type{Kind: "const", Name: "T"}}}}
	if got := ConstraintString(c); got != "true /\\ int ~ a /\\ T is defined" {
		t.Fatal(got)
	}
}
