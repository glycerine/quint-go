package quint

import "testing"

func TestModuleDestructuring(t *testing.T) {
	for _, tc := range []struct {
		source, opcode string
		qualifier      Qualifier
		names          []string
		indexes        []int64
	}{
		{`val (a,_,c) = source()`, "item", Val, []string{"a", "c"}, []int64{1, 3}},
		{`pure val {left,right} = source()`, "field", PureVal, []string{"left", "right"}, nil},
		{`val (_,_) = source()`, "item", Val, nil, nil},
	} {
		ms, err := ParseModules("module M { " + tc.source + " }")
		if err != nil {
			t.Fatal(err)
		}
		ds := ms[0].Declarations
		if len(ds) != len(tc.names)+1 {
			t.Fatalf("declarations %+v", ds)
		}
		temp := ds[0]
		if temp.Name != "quintDestructTemp"+temp.ID.String() || temp.Expr.Opcode != "source" || temp.Qualifier != tc.qualifier {
			t.Fatalf("temporary %+v", temp)
		}
		for i, name := range tc.names {
			d := ds[i+1]
			a := d.Expr
			if d.Name != name || d.Qualifier != tc.qualifier || a.Opcode != tc.opcode || a.Args[0].Name != temp.Name {
				t.Fatalf("field %+v", d)
			}
			if tc.opcode == "item" {
				if a.Args[1].Int.Big().Int64() != tc.indexes[i] {
					t.Fatal("tuple index")
				}
			} else if a.Args[1].Str != name {
				t.Fatal("record field")
			}
		}
	}
}

func TestLocalDestructuring(t *testing.T) {
	for _, source := range []string{`val (a,_,b) = pair; a+b`, `pure val {a,b} = record; a+b`} {
		e, err := ParseExpression(source)
		if err != nil {
			t.Fatal(err)
		}
		if e.Kind != "let" || e.Body.Opdef.Name != "b" || e.Body.Body.Opdef.Name != "a" || e.Body.Body.Body.Opcode != "iadd" {
			t.Fatalf("scope/order %+v", e)
		}
		for _, binding := range []*Expr{e.Body, e.Body.Body} {
			if binding.Opdef.Expr.Args[0].Name != e.Opdef.Name {
				t.Fatal("temporary reference")
			}
		}
	}
	e, err := ParseExpression(`val (_,_) = pair; true`)
	if err != nil {
		t.Fatal(err)
	}
	if e.Body.Kind != "bool" || !e.Body.Bool {
		t.Fatal("all holes")
	}
	nested, err := ParseExpression(`val (a,b) = pair; val {c} = a; c`)
	if err != nil {
		t.Fatal(err)
	}
	inner := nested.Body.Body.Body
	if inner.Kind != "let" || inner.Opdef.Name == nested.Opdef.Name {
		t.Fatal("nested freshness")
	}
	for _, source := range []string{`val (a,) = pair; a`, `val {} = r; true`, `val (a,b) = ; true`} {
		if _, err := ParseExpression(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
}
