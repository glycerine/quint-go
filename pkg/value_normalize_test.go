package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamITFNormalization(t *testing.T) {
	var fixtures []struct {
		Name, Locale string
		Input        json.RawMessage
		Output       []Expr
	}
	data, err := os.ReadFile("testdata/parser/itf_normalize.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			trace, err := DecodeNormalizedITFTrace(f.Input)
			if err != nil {
				t.Fatal(err)
			}
			got, _ := json.Marshal(trace.States)
			want, _ := json.Marshal(f.Output)
			var a, b any
			json.Unmarshal(got, &a)
			json.Unmarshal(want, &b)
			if !reflect.DeepEqual(a, b) {
				t.Fatalf("locale %s\nGo %s\nTS %s", f.Locale, got, want)
			}
			if string(trace.Raw) != string(f.Input) || trace.Loop == nil || *trace.Loop != 0 || len(trace.Metadata) == 0 {
				t.Fatal("trace metadata lost")
			}
			for _, state := range trace.States {
				normalized, err := NormalizeValue(state)
				if err != nil {
					t.Fatal(err)
				}
				again, _ := json.Marshal(normalized)
				before, _ := json.Marshal(state)
				if string(again) != string(before) {
					t.Fatal("normalization is not idempotent")
				}
			}
		})
	}
}

func TestNormalizeValuesAndIdentity(t *testing.T) {
	input, err := ParseExpression(`Map((Set(2,1),1),(Set(1,2),2))`)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(input)
	output, err := NormalizeValue(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(output.Args) != 1 || output.Args[0].Args[1].Int.String() != "2" {
		t.Fatal("last equal key did not win", ExpressionString(output))
	}
	output.Args[0].Args[0].Args[0].Int, _ = ParseInteger("99")
	after, _ := json.Marshal(input)
	if string(before) != string(after) {
		t.Fatal("input changed")
	}
	// Canonically equivalent Unicode strings collate equally, but remain distinct
	// Quint values. Set identity must not depend on their display order.
	input, err = ParseExpression("Set(Set(\"é\",\"é\"),Set(\"é\",\"é\"))")
	if err != nil {
		t.Fatal(err)
	}
	output, err = NormalizeValue(input)
	if err != nil || len(output.Args) != 1 || len(output.Args[0].Args) != 2 {
		t.Fatal(ExpressionString(output), err)
	}
	record, err := ParseExpression(`Rec("z",1,"a",2,"z",3)`)
	if err != nil {
		t.Fatal(err)
	}
	output, err = NormalizeValue(record)
	if err != nil || len(output.Args) != 4 || output.Args[0].Str != "a" || output.Args[3].Int.String() != "3" {
		t.Fatal(output, err)
	}
	for _, opcode := range []string{"Set", "Map", "Rec", "Tup", "List"} {
		output, err = NormalizeValue(Expr{Kind: "app", Opcode: opcode})
		if err != nil || output.Args == nil || len(output.Args) != 0 {
			t.Fatal(output, err)
		}
	}
}

func TestNormalizationRejectsNonValues(t *testing.T) {
	for _, input := range []Expr{
		{Kind: "name", Name: "x"},
		{Kind: "app", Opcode: "iadd"},
		{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}}},
		{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "bool"}, {Kind: "bool"}}},
		{Kind: "app", Opcode: "Map", Args: []Expr{{Kind: "app", Opcode: "Tup"}}},
		{Kind: "app", Opcode: "variant"},
		{Kind: "app", Opcode: "variant", Args: []Expr{{Kind: "bool"}, {Kind: "bool"}}},
	} {
		if _, err := NormalizeValue(input); err == nil {
			t.Fatal("non-value accepted", input)
		}
	}
	if _, err := DecodeNormalizedITFTrace([]byte(`{"states":[{"x":{"#unserializable":"Int"}}]}`)); err == nil {
		t.Fatal("symbolic ITF normalized as concrete")
	}
	if _, err := DecodeNormalizedITFTrace([]byte(`null`)); err == nil {
		t.Fatal("invalid trace accepted")
	}
}
