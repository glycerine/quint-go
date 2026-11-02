package quint

import (
	"reflect"
	"testing"
)

func TestInstanceLinking(t *testing.T) {
	for _, alias := range []bool{false, true} {
		directive, ref := "import A(N=outside).*", "N"
		if alias {
			directive, ref = "import A(N=outside) as I", "I::N"
		}
		ms, err := ParseModules("module A { const N: int val x = N } module B { val outside = 3 " + directive + " val y = " + ref + " }")
		if err != nil {
			t.Fatal(err)
		}
		r, err := ResolveLinkedModules(ms)
		if err != nil || len(r.Errors) != 0 {
			t.Fatalf("%v %+v", err, r.Errors)
		}
		d := ms[1].Declarations[1]
		for _, id := range []ID{d.Overrides[0].Parameter.ID, ms[1].Declarations[2].Expr.ID} {
			got := r.Table.Get(id)
			want := []string{"A", "B"}
			if alias {
				want[0] = "I"
			}
			if got.ID != ms[0].Declarations[0].ID || !reflect.DeepEqual(got.Namespaces, want) || !got.Hidden || got.ImportedFrom.ID != d.ID {
				t.Fatalf("binding %+v", got)
			}
		}
		if r.Table.Get(d.Overrides[0].Expr.ID).ID != ms[1].Declarations[0].ID {
			t.Fatal("override RHS scope")
		}
		if got, _ := r.Modules.Get("A").Lookup("N"); got.Hidden || len(got.Namespaces) != 0 {
			t.Fatal("prototype mutated")
		}
	}
}

func TestInstanceDiagnostics(t *testing.T) {
	cases := []struct {
		source string
		codes  []string
	}{
		{"module A { import A(N=1).* }", []string{QNT407, QNT404}},
		{"module B { import Missing(N=1).* }", []string{QNT405, QNT404}},
		{"module A { val x=1 } module B { import A(N=1).* }", []string{QNT406, QNT404}},
		{"module A { val x=1 } module B { import A(x=missing).* }", []string{QNT406, QNT404}},
	}
	for _, tc := range cases {
		ms, err := ParseModules(tc.source)
		if err != nil {
			t.Fatal(err)
		}
		r, err := ResolveLinkedModules(ms)
		if err != nil {
			t.Fatal(err)
		}
		codes := []string{}
		for _, e := range r.Errors {
			codes = append(codes, e.Code)
		}
		if !reflect.DeepEqual(codes, tc.codes) {
			t.Fatalf("%s: %+v", tc.source, r.Errors)
		}
	}
}

func TestInstanceExportAndIsolation(t *testing.T) {
	ms, err := ParseModules("module A { const N: int val x=N } module P { import A.* } module B { import P(N=1) as I export I.* } module C { import B.* val y=N }")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ResolveLinkedModules(ms)
	if err != nil || len(r.Errors) != 0 {
		t.Fatalf("%v %+v", err, r.Errors)
	}
	proto, _ := r.Modules.Get("P").Lookup("N")
	inst, _ := r.Modules.Get("I").Lookup("N")
	if !proto.Hidden || inst.Hidden {
		t.Fatal("override visibility or copy isolation")
	}
	if r.Table.Get(ms[3].Declarations[1].Expr.ID).ID != ms[0].Declarations[0].ID {
		t.Fatal("exported constant identity")
	}
}
