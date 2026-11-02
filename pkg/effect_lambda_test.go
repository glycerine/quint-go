package quint

import "testing"

func TestEffectLambdaLifecycle(t *testing.T) {
	ids := NewIDGenerator()
	p := Parameter{ID: ids.Next(), Name: "x"}
	body := Expr{Kind: "name", ID: ids.Next(), Name: "x"}
	lambda := Expr{Kind: "lambda", ID: ids.Next(), Params: []Parameter{p}, Body: &body}
	c := NewEffectSignatureContext(nil)
	c.Location = "lambda"
	scopes := EffectLambdaScopes{}
	c.EnterEffectLambda(lambda, &scopes)
	name := "e_x_" + p.ID.String()
	if len(scopes.Frames) != 1 || scopes.Current().Effects[0] != name {
		t.Fatal(scopes)
	}
	c.Save(body.ID, EffectScheme{Effect: Effect{Kind: "variable", Name: name}}, nil)
	read, _ := ParseEffect("Read[r]")
	bindings := []EffectSubstitution{{Kind: "effect", Name: name, Effect: read}}
	c.ExitEffectLambda(lambda, &scopes, bindings)
	s, e := c.Fetch(lambda.ID)
	if e != nil || EffectString(s.Effect) != "(Read[r]) => Read[r]" || len(scopes.Frames) != 0 {
		t.Fatal(s, e, scopes)
	}
	ps, e := c.Fetch(p.ID)
	if e != nil || EffectString(ps.Effect) != "Read[r]" {
		t.Fatal(ps, e)
	}
}
func TestEffectLambdaNestedAndOperatorResult(t *testing.T) {
	ids := NewIDGenerator()
	c := NewEffectSignatureContext(nil)
	c.Location = "nested"
	scopes := EffectLambdaScopes{Frames: []EffectFreeNames{{Effects: []string{"outer"}, Entities: []string{"state"}}}}
	body := Expr{Kind: "name", ID: ids.Next()}
	lambda := Expr{Kind: "lambda", ID: ids.Next(), Params: []Parameter{{ID: ids.Next(), Name: "_"}}, Body: &body}
	c.EnterEffectLambda(lambda, &scopes)
	if len(scopes.Current().Effects) != 2 || scopes.Current().Effects[1] != "_e0" {
		t.Fatal(scopes)
	}
	e, _ := ParseEffect("(a) => a")
	c.Save(body.ID, EffectScheme{Effect: e}, nil)
	c.ExitEffectLambda(lambda, &scopes, nil)
	if _, failure := c.Fetch(body.ID); failure == nil || *failure.Message != "Result cannot be an operator" {
		t.Fatal(failure)
	}
	if _, failure := c.Fetch(lambda.ID); failure != nil {
		t.Fatal(failure)
	}
	if len(scopes.Frames) != 1 || scopes.Current().Effects[0] != "outer" {
		t.Fatal(scopes)
	}
	c.EnterEffectLambda(lambda, &scopes)
	c.ExitEffectLambda(lambda, &scopes, nil)
	if len(scopes.Frames) != 2 {
		t.Fatal("upstream failed-frame behavior changed")
	}
}
