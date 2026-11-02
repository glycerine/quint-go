package quint

import "testing"

func TestModuleExpressionSession(t *testing.T) {
	ids := NewIDGenerator()
	ms, err := parseModulesWithIDs("module A { val x=1 } module B { val x=2 }", ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, err := ResolveLinkedModules(ms)
	if err != nil {
		t.Fatal(err)
	}
	session := linked.ExpressionResolver("A")
	parse := func(text string) *Expr {
		t.Helper()
		v, e := ParseExpressionOrDeclaration(text, ids)
		if e != nil {
			t.Fatal(e)
		}
		return v.Expr
	}
	first := parse("x")
	session.Resolve(first, 0)
	if linked.Table.Get(first.ID).ID != ms[0].Declarations[0].ID {
		t.Fatal("A reference or table sharing")
	}
	session.SwitchToModule("B")
	second := parse("x")
	session.Resolve(second, 0)
	if linked.Table.Get(second.ID).ID != ms[1].Declarations[0].ID || linked.Table.Get(first.ID).ID != ms[0].Declarations[0].ID {
		t.Fatal("switch lost references")
	}
	scoped := parse("List(x).map(x=>x)")
	session.Resolve(scoped, 0)
	if len(session.Errors) != 0 {
		t.Fatal(session.Errors)
	}
	after := parse("x")
	session.Resolve(after, 0)
	if linked.Table.Get(after.ID).ID != ms[1].Declarations[0].ID {
		t.Fatal("lambda scope leaked")
	}
	if len(linked.UnusedDefinitions("A")) != 0 || len(linked.UnusedDefinitions("B")) != 0 {
		t.Fatal("incremental usage missing")
	}
}

func TestModuleSessionMissingScopeAndErrors(t *testing.T) {
	ids := NewIDGenerator()
	ms, err := parseModulesWithIDs("module A { val x=1 } module B { import A as I }", ids)
	if err != nil {
		t.Fatal(err)
	}
	linked, _ := ResolveLinkedModules(ms)
	session := linked.ExpressionResolver("missing")
	expr, err := ParseExpressionOrDeclaration("x", ids)
	if err != nil {
		t.Fatal(err)
	}
	session.Resolve(expr.Expr, 0)
	if len(session.Errors) != 1 || session.Errors[0].Code != QNT404 {
		t.Fatal(session.Errors)
	}
	if linked.Modules.Get("missing") != nil {
		t.Fatal("transient module registered")
	}
	session.SwitchToModule("I")
	if len(session.Errors) != 1 {
		t.Fatal("switch cleared diagnostics")
	}
	session.ClearErrors()
	expr, err = ParseExpressionOrDeclaration("x", ids)
	if err != nil {
		t.Fatal(err)
	}
	session.Resolve(expr.Expr, 0)
	if len(session.Errors) != 0 || linked.Table.Get(expr.Expr.ID).ID != ms[0].Declarations[0].ID {
		t.Fatal("alias scope resolution")
	}
	if len(linked.Errors) != 0 {
		t.Fatal("session diagnostics mutated initial errors")
	}
}

func TestModuleSessionDepthAndRestoration(t *testing.T) {
	ids := NewIDGenerator()
	ms, _ := parseModulesWithIDs("module M { val x=1 }", ids)
	linked, _ := ResolveLinkedModules(ms)
	session := linked.ExpressionResolver("M")
	e, err := parseExpressionWithIDs("{ val x=2; x }", ids)
	if err != nil {
		t.Fatal(err)
	}
	session.Resolve(&e, 3)
	if def := linked.Table.Get(e.Opdef.ID); def.Depth != 5 || !def.Shadowing {
		t.Fatalf("%+v", def)
	}
	def, _ := session.Scope.Lookup("x")
	if def.ID != ms[0].Declarations[0].ID {
		t.Fatal("let scope leaked")
	}
}
