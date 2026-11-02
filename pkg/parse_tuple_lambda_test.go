package quint

import "testing"

func TestTupleLambdaDesugaring(t *testing.T) {
	for _, tc := range []struct {
		source string
		names  []string
	}{
		{`((a,b)) => a + b`, []string{"a", "b"}},
		{`((a,_,c)) => c`, []string{"a", "_", "c"}},
	} {
		e, err := ParseExpression(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		if e.Kind != "lambda" || e.Qualifier != Def || len(e.Params) != 1 {
			t.Fatalf("lambda %+v", e)
		}
		param := e.Params[0]
		if param.Name != "quintTupledLambdaParam"+param.ID.String() {
			t.Fatal("fresh parameter name")
		}
		body := e.Body
		for i := len(tc.names) - 1; i >= 0; i-- {
			if body.Kind != "let" || body.Opdef.Name != tc.names[i] || body.Opdef.Qualifier != PureVal {
				t.Fatalf("binding %d: %+v", i, body)
			}
			access := body.Opdef.Expr
			if access.Opcode != "item" || len(access.Args) != 2 || access.Args[0].Name != param.Name || access.Args[1].Int.Big().Int64() != int64(i+1) {
				t.Fatalf("projection %+v", access)
			}
			body = body.Body
		}
		if body.Kind == "let" {
			t.Fatal("extra binding")
		}
	}
}

func TestTupleLambdaNesting(t *testing.T) {
	e, err := ParseExpression(`((a,b)) => ((c,d)) => a + d`)
	if err != nil {
		t.Fatal(err)
	}
	inner := e.Body.Body.Body
	if inner.Kind != "lambda" || inner.Params[0].Name == e.Params[0].Name {
		t.Fatal("fresh names must differ")
	}
	call, err := ParseExpression(`xs.map(((a,b)) => a + b)`)
	if err != nil {
		t.Fatal(err)
	}
	if call.Opcode != "map" || call.Args[1].Kind != "lambda" {
		t.Fatalf("tuple lambda argument %+v", call)
	}
	for _, source := range []string{`((a)) => a`, `((a,b,)) => a`, `((a,b)) =>`} {
		if _, err := ParseExpression(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}
