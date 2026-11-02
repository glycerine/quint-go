package quint

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestExpressionDecodeTaggedSchemaAndReceiverOwnership(t *testing.T) {
	t.Parallel()
	variants := []struct {
		kind, source string
		required     []string
	}{
		{"name", `{"id":1,"kind":"name","name":"x"}`, []string{"id", "kind", "name"}},
		{"bool", `{"id":1,"kind":"bool","value":false}`, []string{"id", "kind", "value"}},
		{"int", `{"id":1,"kind":"int","value":0}`, []string{"id", "kind", "value"}},
		{"str", `{"id":1,"kind":"str","value":""}`, []string{"id", "kind", "value"}},
		{"app", `{"id":1,"kind":"app","opcode":"List","args":[]}`, []string{"id", "kind", "opcode", "args"}},
		{"lambda", `{"id":1,"kind":"lambda","params":[],"qualifier":"def","expr":{"id":2,"kind":"int","value":0}}`, []string{"id", "kind", "params", "qualifier", "expr"}},
		{"let", `{"id":1,"kind":"let","opdef":{"id":2,"kind":"def","name":"x","qualifier":"val","expr":{"id":3,"kind":"int","value":0}},"expr":{"id":4,"kind":"name","name":"x"}}`, []string{"id", "kind", "opdef", "expr"}},
	}
	for _, variant := range variants {
		t.Run(variant.kind, func(t *testing.T) {
			var valid Expr
			if err := json.Unmarshal([]byte(variant.source), &valid); err != nil || valid.Kind != variant.kind {
				t.Fatal(err, valid)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal([]byte(variant.source), &fields); err != nil {
				t.Fatal(err)
			}
			reject := func(input []byte) {
				t.Helper()
				// Shared mutable subtrees must also survive a failed decode.
				receiver := Expr{Kind: "app", Opcode: "original", Args: []Expr{{Kind: "str", Str: "unchanged"}}}
				before, _ := json.Marshal(receiver)
				if err := json.Unmarshal(input, &receiver); err == nil {
					t.Fatal("accepted invalid expression", string(input))
				}
				after, _ := json.Marshal(receiver)
				if string(before) != string(after) {
					t.Fatal("failed decode mutated receiver", string(input), string(after))
				}
			}
			for _, field := range variant.required {
				original := fields[field]
				delete(fields, field)
				input, _ := json.Marshal(fields)
				reject(input)
				fields[field] = json.RawMessage("null")
				input, _ = json.Marshal(fields)
				reject(input)
				fields[field] = original
			}
			for _, field := range []string{"name", "value", "opcode", "args", "params", "qualifier", "expr", "opdef", "extra", "ID", "Kind", "VALUE", "Args", "typeAnnotation"} {
				if _, required := fields[field]; required {
					continue
				}
				fields[field] = json.RawMessage("null")
				input, _ := json.Marshal(fields)
				reject(input)
				delete(fields, field)
			}
		})
	}
}

func TestExpressionDecodeLegacyJSONAndNestedErrors(t *testing.T) {
	t.Parallel()
	// The map-based decoder accepted duplicate object names and checked their
	// last values. Retain that behavior when decoding through a fixed layout.
	for _, input := range []string{
		`{"id":"bad","id":1,"kind":"int","value":"bad","value":7}`,
		`{"id":1,"kind":42,"kind":"int","value":7}`,
		`{"id":1,"kind":"int","value":0,"\u0076alue":7}`,
	} {
		var got Expr
		if err := json.Unmarshal([]byte(input), &got); err != nil || got.Kind != "int" || got.Int.String() != "7" || got.ID.String() != "1" {
			t.Fatal(input, got, err)
		}
	}
	var escaped Expr
	if err := json.Unmarshal([]byte(`{"\u0069d":0,"kind":"str","value":"x"}`), &escaped); err != nil || escaped.Str != "x" {
		t.Fatal(escaped, err)
	}
	var invalidUTF8 Expr
	if err := json.Unmarshal([]byte("{\"id\":0,\"kind\":\"str\",\"value\":\"\xff\"}"), &invalidUTF8); err != nil || invalidUTF8.Str != "\ufffd" {
		t.Fatal(invalidUTF8, err)
	}
	for _, input := range []string{
		`null`, `[]`, `true`, `{`, `{"id":1,"kind":"int","value":1} {}`,
		`{"id":-1,"kind":"int","value":0}`,
		`{"id":1,"kind":"int","value":1.5}`,
		`{"id":1,"kind":"app","opcode":"List","args":[{"id":2,"kind":"bool","value":null}]}`,
		`{"id":1,"kind":"lambda","params":[{"id":2,"name":"x","extra":true}],"qualifier":"def","expr":{"id":3,"kind":"int","value":0}}`,
		`{"id":1,"kind":"lambda","params":[],"qualifier":"unknown","expr":{"id":2,"kind":"int","value":0}}`,
		`{"id":1,"kind":"let","opdef":{"id":2,"kind":"def","name":"x","qualifier":"val","expr":null},"expr":{"id":4,"kind":"int","value":0}}`,
	} {
		receiver := Expr{Kind: "str", Str: "original"}
		before := receiver
		if err := json.Unmarshal([]byte(input), &receiver); err == nil || !reflect.DeepEqual(before, receiver) {
			t.Fatal("malformed expression accepted or receiver changed", input, err, receiver)
		}
	}
}
