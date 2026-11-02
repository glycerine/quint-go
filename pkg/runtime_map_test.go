package quint

import "testing"

func TestMapValidationAndOwnership(t *testing.T) {
	makeValue := func(source string) RuntimeValue {
		t.Helper()
		e, err := ParseExpression(source)
		if err != nil {
			t.Fatal(err)
		}
		v, err := NewRuntimeValue(e)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	e, _ := ParseExpression("Map((2,20),(1,10))")
	m, err := NewRuntimeValue(e)
	if err != nil {
		t.Fatal(err)
	}
	e.Args[0].Args[1] = Expr{Kind: "str", Str: "changed"}
	key, value := makeValue("1"), makeValue("99")
	before := m.String()
	for _, op := range []string{"set", "put"} {
		out, err := EvalMapBuiltin(op, []RuntimeValue{m, key, value})
		if err != nil || out.Equal(m) || m.String() != before {
			t.Fatal(op, out.String(), err)
		}
	}
	got, err := EvalMapBuiltin("get", []RuntimeValue{m, key})
	if err != nil || got.String() != "10" {
		t.Fatal(got.String(), err)
	}
	for _, c := range []struct {
		op   string
		args []RuntimeValue
	}{{"unknown", nil}, {"get", nil}, {"get", []RuntimeValue{{}, key}}, {"get", []RuntimeValue{key, key}}, {"put", []RuntimeValue{m, key, {}}}} {
		if _, err := EvalMapBuiltin(c.op, c.args); err == nil {
			t.Fatal("invalid call accepted", c.op)
		}
	}
}
