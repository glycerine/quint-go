package quint

import "testing"

func TestConvertInit(t *testing.T) {
	for _, shared := range []bool{false, true} {
		tail := "action step=x'=2"
		if shared {
			tail = "action step=start"
		}
		ms, err := ParseModules("module M { var x:int action start=x'=1 action q::init=start " + tail + " }")
		if err != nil {
			t.Fatal(err)
		}
		linked, _ := ResolveLinkedModules(ms)
		out, errors := ConvertInit(ms[0], linked.Table, nil)
		if shared {
			if len(errors) != 1 || errors[0].Code != "QNT409" {
				t.Fatal(errors)
			}
			continue
		}
		if len(errors) != 0 || out.Declarations[1].Expr.Opcode != "eq" || out.Declarations[3].Expr.Opcode != "assign" {
			t.Fatal(out, errors)
		}
		if ms[0].Declarations[1].Expr.Opcode != "assign" {
			t.Fatal("input mutated")
		}
	}
}
