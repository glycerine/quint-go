package quint

import "testing"

func TestRefreshDefinitionIDs(t *testing.T) {
	ids := NewIDGenerator()
	ms, err := parseModulesWithIDs("module M { def f(x:int):int=val local=1; x+local }", ids)
	if err != nil {
		t.Fatal(err)
	}
	d := ms[0].Declarations[0]
	out, err := RefreshDefinitionIDs(d, ids, nil, AnalysisOutput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.ID == d.ID || out.TypeAnnotation.Res.ID == nil || *out.TypeAnnotation.Res.ID == *d.TypeAnnotation.Res.ID {
		t.Fatal("definition annotation not refreshed")
	}
	if out.Expr.Params[0].ID == d.Expr.Params[0].ID || *out.Expr.Params[0].TypeAnnotation.ID == *d.Expr.Params[0].TypeAnnotation.ID {
		t.Fatal("parameter annotation not refreshed")
	}
	if out.Expr.Body.Opdef.ID == d.Expr.Body.Opdef.ID {
		t.Fatal("local ID retained")
	}
	if ExpressionString(*out.Expr) != ExpressionString(*d.Expr) {
		t.Fatal("meaning changed")
	}
}
