package quint

import "testing"

func TestEffectApplicationInference(t *testing.T) {
	for _, tc := range []struct {
		opcode string
		inputs []string
		want   string
		fails  bool
	}{
		{"iadd", []string{"Pure", "Read['x']"}, "Read['x']", false},
		{"assign", []string{"Read['x']", "Read['y']"}, "Read['y'] & Update['x']", false},
		{"next", []string{"Read['x']"}, "Temporal['x']", false},
		{"field", []string{"Read['x']", "Update['y']"}, "", true},
		{"List", nil, "Pure", false},
	} {
		t.Run(tc.opcode, func(t *testing.T) {
			c := NewEffectSignatureContext(nil)
			expr := Expr{Kind: "app", Opcode: tc.opcode, ID: NewIDGenerator().Next()}
			ids := NewIDGenerator()
			ids.Next()
			for _, text := range tc.inputs {
				id := ids.Next()
				expr.Args = append(expr.Args, Expr{Kind: "bool", ID: id})
				e, err := ParseEffect(text)
				if err != nil {
					t.Fatal(err)
				}
				c.Save(id, EffectScheme{Effect: e}, nil)
			}
			bindings := c.InferApplication(expr, nil)
			s, e := c.Fetch(expr.ID)
			if tc.fails {
				if e == nil {
					t.Fatal("expected error")
				}
				return
			}
			if e != nil || EffectString(s.Effect) != tc.want {
				t.Fatalf("%s %v", EffectString(s.Effect), e)
			}
			if len(bindings) == 0 {
				t.Fatal("missing bindings")
			}
			for _, a := range expr.Args {
				s, e = c.Fetch(a.ID)
				if e != nil || len(s.EffectVariables) > 0 || len(s.EntityVariables) > 0 {
					t.Fatal(s, e)
				}
			}
		})
	}
}
