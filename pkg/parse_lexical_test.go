package quint

import (
	"encoding/json"
	"testing"
)

func TestIntegerLiteralSyntax(t *testing.T) {
	for _, tc := range [][2]string{
		{"0", "0"}, {"1_000_001", "1000001"}, {"0x0", "0"}, {"0xAb_CD", "43981"},
		{"0xffff_ffff_ffff_ffff_ffff_ffff_ffff_ffff", "340282366920938463463374607431768211455"},
	} {
		e, err := ParseExpression(tc[0])
		if err != nil {
			t.Fatal(err)
		}
		if e.Kind != "int" || e.Int.String() != tc[1] {
			t.Fatalf("%s: %+v", tc[0], e)
		}
		data, err := json.Marshal(e.Int)
		if err != nil || string(data) != tc[1] {
			t.Fatalf("JSON %s %v", data, err)
		}
	}
	e, err := ParseExpression("-0xFF")
	if err != nil || e.Opcode != "iuminus" || e.Args[0].Int.String() != "255" {
		t.Fatalf("negative hex %+v %v", e, err)
	}
	for _, source := range []string{"00", "01", "0_1", "1__2", "1_", "0x", "0x_1", "0x1_", "0x1__2", "0xGG", "0XFF", "0b10", "1.5"} {
		if _, err := ParseExpression(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
	var n Integer
	if err := json.Unmarshal([]byte("0x10"), &n); err == nil {
		t.Fatal("accepted hexadecimal JSON")
	}
}

func TestLexicalModuleBoundaries(t *testing.T) {
	ms, err := ParseModules("#!/usr/bin/env quint\n/* header */ module A::B { // line\n val n = 0x10\n }")
	if err != nil || ms[0].Name != "A::B" || ms[0].Declarations[0].Expr.Int.String() != "16" {
		t.Fatalf("module %v %v", ms, err)
	}
	for _, source := range []string{
		"module M {}\n#!/usr/bin/env quint\nmodule N {}",
		"module M { val if = 1 }", "module M { val x = 1 /* unterminated }",
	} {
		if _, err := ParseModules(source); err == nil {
			t.Errorf("accepted %q", source)
		}
	}
	for _, source := range []string{"A::B::x", "_name", "name_1"} {
		e, err := ParseExpression(source)
		if err != nil || e.Kind != "name" || e.Name != source {
			t.Fatalf("name %q: %+v %v", source, e, err)
		}
	}
}
