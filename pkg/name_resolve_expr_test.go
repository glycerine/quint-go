package quint

import "testing"

func TestExpressionNameResolution(t *testing.T) {
	modules, err := ParseModules(`module M { val x = 1 def f(a) = val x = a; x + f(a) }`)
	if err != nil {
		t.Fatal(err)
	}
	scope := CollectLocalNames(modules).Modules.Get("M")
	r := NewExpressionNameResolver(scope)
	lambda := modules[0].Declarations[1].Expr
	r.Resolve(lambda, 0)
	if len(r.Errors) != 0 || len(scope.Errors) != 0 {
		t.Fatalf("errors %v %v", r.Errors, scope.Errors)
	}
	let := lambda.Body
	if got := r.Table.Get(let.Opdef.Expr.ID); got.ID != lambda.Params[0].ID || got.Depth != 1 {
		t.Fatal("parameter resolution")
	}
	if got := r.Table.Get(let.Body.Args[0].ID); got.ID != let.Opdef.ID || got.Depth != 3 || !got.Shadowing {
		t.Fatal("local binding resolution")
	}
	if got := r.Table.Get(let.Body.Args[1].ID); got.ID != modules[0].Declarations[1].ID {
		t.Fatal("recursive reference")
	}
	if got, _ := scope.Lookup("x"); got.ID != modules[0].Declarations[0].ID {
		t.Fatal("outer scope not restored")
	}
	if _, ok := scope.Lookup("a"); ok {
		t.Fatal("parameter leaked")
	}
}
func TestExpressionUndefinedAndTypeNames(t *testing.T) {
	ms, err := ParseModules(`module M { type T = int val x = 1 }`)
	if err != nil {
		t.Fatal(err)
	}
	scope := CollectLocalNames(ms).Modules.Get("M")
	r := NewExpressionNameResolver(scope)
	expr, err := ParseExpression(`missing(T, x, true)`)
	if err != nil {
		t.Fatal(err)
	}
	r.Resolve(&expr, 0)
	if len(r.Errors) != 2 || r.Errors[0].Message != "Name 'missing' not found" || r.Errors[1].Message != "Name 'T' not found" {
		t.Fatalf("errors %+v", r.Errors)
	}
	if r.Table.Get(expr.Args[1].ID).ID != ms[0].Declarations[1].ID {
		t.Fatal("valid argument not resolved")
	}
	typ, _, err := ParseTypeWithSource("t", "T -> x")
	if err != nil {
		t.Fatal(err)
	}
	// Lowercase x is a type variable, not a named alias, so it needs no lookup.
	r.resolveType(&typ)
	if r.Table.Get(*typ.Arg.ID).ID != ms[0].Declarations[0].ID {
		t.Fatal("alias reference")
	}
}
