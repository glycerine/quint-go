package quint

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamIntegerBuiltins(t *testing.T) {
	var cases []struct {
		Op     string
		Args   []string
		Output *Expr
		Error  *Diagnostic
		Thrown string
	}
	data, err := os.ReadFile("testdata/parser/runtime_integer.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.Op+"/"+joinIntegerArgs(c.Args), func(t *testing.T) {
			args := make([]RuntimeValue, len(c.Args))
			for i, text := range c.Args {
				n, err := ParseInteger(text)
				if err != nil {
					t.Fatal(err)
				}
				args[i], err = NewRuntimeValue(Expr{Kind: "int", Int: n})
				if err != nil {
					t.Fatal(err)
				}
			}
			before := make([]string, len(args))
			for i, a := range args {
				before[i] = a.String()
			}
			got, err := EvalIntegerBuiltin(c.Op, args)
			for i, a := range args {
				if a.String() != before[i] {
					t.Fatal("argument mutated")
				}
			}
			if c.Thrown != "" {
				// TypeScript imod(0) throws an uncaught BigInt RangeError. Go converts
				// this to the same structured diagnostic used by division by zero.
				if c.Op != "imod" || c.Thrown != "Division by zero" {
					t.Fatal("unexpected upstream exception", c.Thrown)
				}
				var d Diagnostic
				if !errors.As(err, &d) || d.Code != "QNT503" || d.Message != "Division by zero" {
					t.Fatal(err)
				}
				return
			}
			if c.Error != nil {
				var d Diagnostic
				if !errors.As(err, &d) || d.Code != c.Error.Code || d.Message != c.Error.Message {
					t.Fatal(err, c.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want, err := NewRuntimeValue(*c.Output)
			if err != nil || !got.Equal(want) {
				t.Fatal(got.String(), want.String(), err)
			}
		})
	}
}
func joinIntegerArgs(args []string) string {
	out := ""
	for _, a := range args {
		out += a + ","
	}
	return out
}

func TestIntegerBuiltinValidation(t *testing.T) {
	one, _ := NewRuntimeValue(Expr{Kind: "int"})
	boolean, _ := NewRuntimeValue(Expr{Kind: "bool"})
	for _, c := range []struct {
		op   string
		args []RuntimeValue
	}{{"unknown", nil}, {"iuminus", nil}, {"iadd", []RuntimeValue{one}}, {"iadd", []RuntimeValue{one, one, one}}, {"iadd", []RuntimeValue{one, boolean}}, {"iuminus", []RuntimeValue{{}}}} {
		_, err := EvalIntegerBuiltin(c.op, c.args)
		var d Diagnostic
		if !errors.As(err, &d) || d.Code != "QNT501" {
			t.Fatal(c.op, err)
		}
	}
}
