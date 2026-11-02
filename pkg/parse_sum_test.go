package quint

import (
	"encoding/json"
	"testing"
)

func TestSumDeclarationConstructors(t *testing.T) {
	ms, err := ParseModules(`module M {
 /// Result docs
 type Result[a,b] = Ok(a) | Err(b) | Pending
 val next = Pending
 }`)
	if err != nil {
		t.Fatal(err)
	}
	ds := ms[0].Declarations
	if len(ds) != 5 {
		t.Fatalf("declarations: %+v", ds)
	}
	d := ds[0]
	if d.Type.Kind != "sum" || len(d.Type.Fields.Fields) != 3 || len(d.Params) != 2 || d.Params[0] != "a" || d.Params[1] != "b" || ds[3].Doc == nil || *ds[3].Doc != "Result docs" {
		t.Fatalf("type: %+v", d)
	}
	for i, name := range []string{"Ok", "Err", "Pending"} {
		c := ds[i+1]
		if c.Name != name || c.Kind != "def" || (i < 2 && c.Doc != nil) {
			t.Fatalf("constructor %+v", c)
		}
		expr, ret := c.Expr, c.TypeAnnotation
		if i < 2 {
			if c.Qualifier != Def || expr.Kind != "lambda" || expr.Qualifier != Def || len(expr.Params) != 1 || expr.Params[0].Name != "__"+name+"Param" {
				t.Fatalf("lambda %+v", expr)
			}
			if ret.Kind != "oper" || len(ret.Args) != 1 || ret.Args[0].Name != d.Params[i] {
				t.Fatalf("annotation %+v", ret)
			}
			expr, ret = expr.Body, ret.Res
			if expr.Args[1].Kind != "name" || expr.Args[1].Name != c.Expr.Params[0].Name {
				t.Fatal("payload binding")
			}
		} else {
			if c.Qualifier != Val || expr.Args[1].Opcode != "Tup" || len(expr.Args[1].Args) != 0 {
				t.Fatal("unit constructor")
			}
		}
		if ret.Kind != "app" || ret.Ctor.Name != "Result" || len(ret.Args) != 2 || ret.Args[0].Name != "a" || ret.Args[1].Name != "b" {
			t.Fatalf("return type %+v", ret)
		}
		if expr.Kind != "app" || expr.Opcode != "variant" || len(expr.Args) != 2 || expr.Args[0].Str != name {
			t.Fatalf("variant %+v", expr)
		}
	}
	data, err := json.Marshal(ms)
	if err != nil {
		t.Fatal(err)
	}
	var roundtrip []Module
	if err = json.Unmarshal(data, &roundtrip); err != nil {
		t.Fatal(err)
	}
}

func TestSumAliasDisambiguation(t *testing.T) {
	for _, tc := range []struct {
		source, kind string
		count        int
	}{
		{`type T = A`, "const", 1},
		{`type T = | A`, "sum", 2},
		{`type T = A(int)`, "sum", 2},
		{`type T = A | B`, "sum", 3},
		{`type T = | A(())`, "sum", 2},
	} {
		t.Run(tc.source, func(t *testing.T) {
			ms, err := ParseModules("module M { " + tc.source + " }")
			if err != nil {
				t.Fatal(err)
			}
			ds := ms[0].Declarations
			if len(ds) != tc.count || ds[0].Type.Kind != tc.kind {
				t.Fatalf("declarations %+v", ds)
			}
			if tc.kind == "const" && ds[0].Type.Name != "A" {
				t.Fatal("alias target")
			}
			if len(ds) > 1 {
				typ := ds[1].TypeAnnotation
				if typ.Kind == "oper" {
					typ = typ.Res
				}
				if typ.Kind != "const" || typ.Name != "T" {
					t.Fatal("unparameterized result")
				}
			}
		})
	}
	for _, s := range []string{`type T = |`, `type T = A(int) |`, `type T = A(,)`} {
		if _, err := ParseModules("module M { " + s + " }"); err == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
