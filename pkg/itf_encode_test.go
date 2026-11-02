package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestUpstreamITFEncoding(t *testing.T) {
	var fixtures []struct {
		Name   string
		Input  []Expr
		Vars   []string
		Mbt    bool
		Output json.RawMessage
		Error  string
	}
	data, err := os.ReadFile("testdata/parser/itf_encode.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			before, _ := json.Marshal(f.Input)
			result, err := EncodeITFTrace(f.Vars, f.Input, f.Mbt)
			if f.Error != "" {
				if err == nil || err.Error() != f.Error {
					t.Fatal(err, f.Error)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			json.Unmarshal(result, &got)
			json.Unmarshal(f.Output, &want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Go %s\nTS %s", result, f.Output)
			}
			after, _ := json.Marshal(f.Input)
			if string(before) != string(after) {
				t.Fatal("input changed")
			}
			trace, err := DecodeITFTrace(result)
			if err != nil || len(trace.States) != len(f.Input) {
				t.Fatal(trace, err)
			}
		})
	}
}

func TestITFValueEncodingOrderAndPrecision(t *testing.T) {
	expression, err := ParseExpression(`Rec("b",9007199254740993123456789,"10",1,"2",2,"b",3,"__private",true)`)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := EncodeITFValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"2":{"#bigint":"2"},"10":{"#bigint":"1"},"b":{"#bigint":"3"},"__private":true}`
	if string(encoded) != want {
		t.Fatal(string(encoded))
	}
	number, _ := ParseInteger("-9007199254740993123456789")
	encoded, err = EncodeITFValue(Expr{Kind: "int", Int: number})
	if err != nil || string(encoded) != `{"#bigint":"-9007199254740993123456789"}` {
		t.Fatal(string(encoded), err)
	}
	for _, opcode := range []string{"List", "Set", "Tup", "Map", "Rec"} {
		encoded, err = EncodeITFValue(Expr{Kind: "app", Opcode: opcode})
		if err != nil || strings.Contains(string(encoded), "null") {
			t.Fatal(string(encoded), err)
		}
	}
	vars := []string{ITFActionTaken}
	trace, err := EncodeITFTrace(vars, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Vars   []string
		States []json.RawMessage
	}
	if err = json.Unmarshal(trace, &value); err != nil {
		t.Fatal(err)
	}
	if len(vars) != 1 || !reflect.DeepEqual(value.Vars, []string{ITFActionTaken, ITFActionTaken, ITFNondetPicks}) || value.States == nil {
		t.Fatal(string(trace))
	}
}

func TestITFEncodingMalformedConstructors(t *testing.T) {
	for _, value := range []Expr{
		{Kind: "name", Name: "x"},
		{Kind: "app", Opcode: "iadd"},
		{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}}},
		{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "bool", Bool: true}, {Kind: "bool"}}},
		{Kind: "app", Opcode: "Map", Args: []Expr{{Kind: "str", Str: "x"}}},
		{Kind: "app", Opcode: "Map", Args: []Expr{{Kind: "app", Opcode: "Tup", Args: []Expr{{Kind: "bool"}}}}},
		{Kind: "app", Opcode: "variant"},
		{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "name", Name: "x"}}},
	} {
		if _, err := EncodeITFValue(value); err == nil {
			t.Fatal("malformed constructor accepted", value)
		}
	}
	if _, err := EncodeITFTrace(nil, []Expr{{Kind: "str", Str: "state"}}, false); err == nil || err.Error() != "Expected a valid ITF state, but found state" {
		t.Fatal(err)
	}
}
