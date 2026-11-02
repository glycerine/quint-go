package quint

import "testing"

func TestVariantConstraintsDeclaredType(t *testing.T) {
	ms, err := ParseModules("module M { type Result=Ok(int)|Err(str) }")
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	id, _ := ParseID("999")
	result := Type{Kind: "var", Name: "r"}
	one, _ := ParseInteger("1")
	for _, tc := range []struct {
		label   string
		payload Type
		success bool
	}{
		{"Ok", Type{Kind: "int"}, true},
		{"Err", Type{Kind: "str"}, true},
		{"Ok", Type{Kind: "bool"}, false},
		{"Missing", Type{Kind: "int"}, false},
	} {
		args := []TypedExpression{{Expr: Expr{Kind: "str", Str: tc.label}, Type: Type{Kind: "str"}}, {Expr: Expr{Kind: "int", Int: one}, Type: tc.payload}}
		cs, e := VariantConstraints(id, args, result)
		if e != nil {
			t.Fatal(e)
		}
		if cs[0].Kind != "isDefined" || cs[1].Kind != "eq" || cs[0].SourceID != id {
			t.Fatal("constraint shape")
		}
		solved, err := (TypeUnifier{Table: linked.Table}).SolveConstraint(Constraint{Kind: "conjunction", Constraints: cs})
		if err != nil {
			t.Fatal(err)
		}
		failures := 0
		for range solved.Errors.All() {
			failures++
		}
		if (failures == 0) != tc.success {
			t.Fatalf("%s: %+v", tc.label, solved)
		}
		if tc.success {
			typ, err := ApplySubstitution(solved.Substitutions, result)
			if err != nil {
				t.Fatal(err)
			}
			typ = SimplifyType(typ)
			if typ.Kind != "sum" || len(typ.Fields.Fields) != 2 || typ.Fields.Other.Kind != "empty" {
				t.Fatalf("%+v", typ)
			}
		}
	}
}
func TestVariantConstraintDiagnostics(t *testing.T) {
	id, _ := ParseID("1")
	result := Type{Kind: "var", Name: "r"}
	args := []TypedExpression{{Expr: Expr{Kind: "name", Name: "label"}, Type: Type{Kind: "int"}}, {Expr: Expr{Kind: "bool", Bool: true}, Type: Type{Kind: "bool"}}}
	_, e := VariantConstraints(id, args, result)
	if e == nil || *e.Message != "Variant label must be a string expression but is int: label" || e.Location != "Generating variant constraints for label,true" {
		t.Fatal(e)
	}
	if _, e = VariantConstraints(id, nil, result); e == nil {
		t.Fatal("arity")
	}
	args[0] = TypedExpression{Expr: Expr{Kind: "str", Str: "Ok"}, Type: Type{Kind: "str"}}
	cs, e := VariantConstraints(id, args, result)
	if e != nil {
		t.Fatal(e)
	}
	solved, err := (TypeUnifier{}).SolveConstraint(Constraint{Kind: "conjunction", Constraints: cs})
	if err != nil {
		t.Fatal(err)
	}
	if e := solved.Errors.Get("1"); e.Message == nil || *e.Message != "Expected type is not defined" {
		t.Fatal(e)
	}
}
