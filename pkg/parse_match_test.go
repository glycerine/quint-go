package quint

import "testing"

func TestParseMatchCases(t *testing.T) {
	for _, pipe := range []string{"", "|"} {
		e, err := ParseExpression(`match result { ` + pipe + ` Ok(value) => value + 1 | Pending => 0 | Err(_) => -1 | _ => 42 }`)
		if err != nil {
			t.Fatal(err)
		}
		if e.Kind != "app" || e.Opcode != "matchVariant" || len(e.Args) != 9 || e.Args[0].Name != "result" {
			t.Fatalf("match %+v", e)
		}
		for i, label := range []string{"Ok", "Pending", "Err", "_"} {
			l, fn := e.Args[1+2*i], e.Args[2+2*i]
			if l.Kind != "str" || l.Str != label || fn.Kind != "lambda" || fn.Qualifier != Def || len(fn.Params) != 1 {
				t.Fatalf("arm %d: %+v %+v", i, l, fn)
			}
			name := "_"
			if i == 0 {
				name = "value"
			}
			if fn.Params[0].Name != name || fn.Body == nil {
				t.Fatalf("binding %+v", fn)
			}
		}
		if e.Args[2].Body.Opcode != "iadd" || e.Args[2].Body.Args[0].Name != "value" {
			t.Fatal("payload reference")
		}
		seen := map[ID]bool{}
		err = WalkExpr(&e, func(node *Expr) (bool, error) {
			if seen[node.ID] {
				t.Errorf("duplicate expression ID %v", node.ID)
			}
			seen[node.ID] = true
			for _, param := range node.Params {
				if seen[param.ID] {
					t.Errorf("duplicate parameter ID %v", param.ID)
				}
				seen[param.ID] = true
			}
			return true, nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestNestedMatchAndModuleIntegration(t *testing.T) {
	ms, err := ParseModules(`module M {
 type Option[a] = Some(a) | None
 pure def value(o) = match o { Some(x) => match x { A(y) => y | _ => 0 } | None => 0 }
 }`)
	if err != nil {
		t.Fatal(err)
	}
	body := ms[0].Declarations[3].Expr.Body
	if body.Opcode != "matchVariant" || body.Args[2].Body.Opcode != "matchVariant" {
		t.Fatalf("nested match %+v", body)
	}
	for _, source := range []string{`match x {}`, `match x { A(v) => }`, `match x { A(x,y) => x }`, `match x { A => 1 | }`} {
		if _, err := ParseExpression(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}
