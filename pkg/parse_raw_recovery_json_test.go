package quint

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRawRecoveredModulesValidWire(t *testing.T) {
	t.Parallel()
	for _, wire := range []string{
		`null`, `[]`,
		`[{"id":1,"name":"M","declarations":[],"doc":""}]`,
		`[{"id":9007199254740993123456789,"name":"M😀","doc":"", "declarations":[
		 {"id":1,"kind":"def","name":"f","qualifier":"val","doc":"","namespaces":[],"hidden":false,"depth":0,"shadowing":{},"importedFrom":{},"typeAnnotation":{"kind":"int"},"expr":
		  {"id":2,"kind":"app","opcode":"List","args":[
		   {"id":3,"kind":"name","name":"x"},{"id":4,"kind":"bool","value":false},
		   {"id":5,"kind":"int","value":9007199254740993123456789},{"id":6,"kind":"str","value":"😀\u0000"},
		   {"id":7,"kind":"app","opcode":"Set","args":[]},
		   {"id":8,"kind":"lambda","qualifier":"def","params":[{"id":9,"name":"p","depth":0,"kind":"param","shadowing":{},"typeAnnotation":{"kind":"str"}}],"expr":{"id":10,"kind":"name","name":"p"}},
		   {"id":11,"kind":"lambda","qualifier":"def","params":[],"expr":{"id":12,"kind":"bool","value":true}},
		   {"id":13,"kind":"let","opdef":{"id":14,"kind":"def","name":"g","qualifier":"val","doc":"","namespaces":["M"],"hidden":false,"depth":0,"shadowing":{},"importedFrom":{},"typeAnnotation":{"kind":"int"},"expr":{"id":15,"kind":"int","value":2}},"expr":{"id":16,"kind":"name","name":"g"}}
		  ]}},
		 {"id":17,"kind":"const","name":"C","typeAnnotation":{"kind":"int"}},
		 {"id":18,"kind":"var","name":"v","typeAnnotation":{"kind":"str"}},
		 {"id":19,"kind":"assume","name":"_","assumption":{"id":20,"kind":"bool","value":true}},
		 {"id":21,"kind":"typedef","name":"T","params":[],"type":{"kind":"rec","fields":{"kind":"empty"}}},
		 {"id":22,"kind":"typedef","name":"U","params":["a"],"type":{"kind":"var","name":"a"}},
		 {"id":23,"kind":"typedef","name":"Uninterpreted"},
		 {"id":24,"kind":"import","protoName":"N","defName":"*","fromSource":"n","qualifiedName":""},
		 {"id":25,"kind":"export","protoName":"N","namespaces":[]},
		 {"id":26,"kind":"instance","protoName":"N","overrides":[],"identityOverride":false},
		 {"id":27,"kind":"instance","protoName":"N","overrides":[[{"id":28,"name":"C"},{"id":29,"kind":"int","value":3}]],"identityOverride":true}
		]}]`,
	} {
		var modules []Module
		if err := json.Unmarshal([]byte(wire), &modules); err != nil {
			t.Fatal(err)
		}
		want, err := json.Marshal(modules)
		if err != nil {
			t.Fatal(err)
		}
		got, err := marshalRawRecoveredModules(modules)
		if err != nil {
			t.Fatal(err)
		}
		assertRawRecoveryJSONEqual(t, got, want)
		var roundtrip []Module
		if err := json.Unmarshal(got, &roundtrip); err != nil || !reflect.DeepEqual(modules, roundtrip) {
			t.Fatalf("valid recovery wire changed strict IR: %v", err)
		}
	}
	// The ordinary encoder preserves nil declarations even though the strict
	// decoder rejects them; raw serialization must preserve that boundary too.
	modules := []Module{{ID: rawRecoveryTestID("1"), Name: "M"}}
	got, err := marshalRawRecoveredModules(modules)
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(modules)
	if err != nil {
		t.Fatal(err)
	}
	assertRawRecoveryJSONEqual(t, got, want)
	var strict []Module
	if err := json.Unmarshal(got, &strict); err == nil {
		t.Fatal("strict module decoder admitted nil declarations")
	}
}

func TestRawRecoveredModulesNullQuarantine(t *testing.T) {
	t.Parallel()
	null := Expr{ID: rawRecoveryTestID("999"), Kind: rawRecoveryNullKind, Name: "must not escape"}
	operator := OperatorDef{ID: rawRecoveryTestID("3"), Kind: "def", Name: "local", Qualifier: Val, Expr: &null}
	let := Expr{ID: rawRecoveryTestID("2"), Kind: "let", Opdef: &operator, Body: &null}
	lambda := Expr{ID: rawRecoveryTestID("4"), Kind: "lambda", Qualifier: Def, Body: &let}
	root := Expr{ID: rawRecoveryTestID("1"), Kind: "app", Opcode: "Rec", Args: []Expr{null, lambda}}
	modules := []Module{{ID: rawRecoveryTestID("5"), Name: "M", Declarations: []Declaration{
		{ID: rawRecoveryTestID("6"), Kind: "def", Name: "r", Qualifier: Val, Expr: &root},
		{ID: rawRecoveryTestID("7"), Kind: "assume", Name: "_", Assumption: &null},
		{ID: rawRecoveryTestID("8"), Kind: "instance", ProtoName: "N", Overrides: []Override{{Parameter: Parameter{ID: rawRecoveryTestID("9"), Name: "C"}, Expr: root}}},
	}}}
	before := jsonIRFuzzIdentities(reflect.ValueOf(modules))
	got, err := marshalRawRecoveredModules(modules)
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"id":5,"name":"M","declarations":[
	 {"id":6,"kind":"def","name":"r","qualifier":"val","expr":{"id":1,"kind":"app","opcode":"Rec","args":[null,{"id":4,"kind":"lambda","qualifier":"def","params":[],"expr":{"id":2,"kind":"let","opdef":{"id":3,"kind":"def","name":"local","qualifier":"val","expr":null},"expr":null}}]}},
	 {"id":7,"kind":"assume","name":"_","assumption":null},
	 {"id":8,"kind":"instance","protoName":"N","overrides":[[{"id":9,"name":"C"},{"id":1,"kind":"app","opcode":"Rec","args":[null,{"id":4,"kind":"lambda","qualifier":"def","params":[],"expr":{"id":2,"kind":"let","opdef":{"id":3,"kind":"def","name":"local","qualifier":"val","expr":null},"expr":null}}]}]],"identityOverride":false}
	]}]`
	assertRawRecoveryJSONEqual(t, got, []byte(want))
	if !reflect.DeepEqual(before, jsonIRFuzzIdentities(reflect.ValueOf(modules))) || null.Kind != rawRecoveryNullKind || null.ID != rawRecoveryTestID("999") || null.Name != "must not escape" {
		t.Fatal("raw serializer changed its input")
	}
	for _, value := range []any{null, root, modules} {
		if _, err := json.Marshal(value); err == nil {
			t.Fatalf("strict encoder admitted recovery-only null: %T", value)
		}
	}
	var decoded []Module
	if err := json.Unmarshal(got, &decoded); err == nil {
		t.Fatal("strict module decoder admitted raw recovery operands")
	}
	if bytes.Contains(got, []byte("parser-null")) || bytes.Contains(got, []byte("999")) {
		t.Fatal("sentinel acquired a wire tag or expression ID")
	}
}

func TestRawRecoveredModulesRejectInvalidIR(t *testing.T) {
	t.Parallel()
	leaf := Expr{Kind: "bool"}
	for name, expression := range map[string]Expr{
		"unknown":       {Kind: "unknown"},
		"sentinel typo": {Kind: "parser-null"},
		"nested kind":   {Kind: "app", Args: []Expr{{Kind: "unknown"}}},
		"lambda body":   {Kind: "lambda", Qualifier: Def},
		"lambda mode":   {Kind: "lambda", Qualifier: "unknown", Body: &leaf},
		"let body":      {Kind: "let", Opdef: &OperatorDef{Kind: "def"}},
		"let operator":  {Kind: "let", Body: &leaf},
		"operator kind": {Kind: "let", Body: &leaf, Opdef: &OperatorDef{Kind: "unknown", Expr: &leaf}},
	} {
		t.Run(name, func(t *testing.T) {
			modules := []Module{{Declarations: []Declaration{{Kind: "def", Name: "f", Qualifier: Val, Expr: &expression}}}}
			if data, err := marshalRawRecoveredModules(modules); err == nil || data != nil {
				t.Fatalf("invalid expression accepted: %s, %v", data, err)
			}
		})
	}
	for _, declaration := range []Declaration{{Kind: "unknown"}, {Kind: "def", Name: "f", Qualifier: Val}} {
		if data, err := marshalRawRecoveredModules([]Module{{Declarations: []Declaration{declaration}}}); err == nil || data != nil {
			t.Fatalf("invalid declaration accepted: %s, %v", data, err)
		}
	}
	invalidMetadata := Declaration{Kind: "def", Name: "f", Qualifier: Val, Expr: &leaf, Shadowing: json.RawMessage("invalid")}
	if _, err := marshalRawRecoveredModules([]Module{{Declarations: []Declaration{invalidMetadata}}}); err == nil {
		t.Fatal("invalid delegated JSON metadata accepted")
	}
}

func assertRawRecoveryJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	decode := func(data []byte) any {
		t.Helper()
		decoder := json.NewDecoder(strings.NewReader(string(data)))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	if !reflect.DeepEqual(decode(got), decode(want)) {
		t.Errorf("raw recovery wire\n got %s\nwant %s", got, want)
	}
}

func rawRecoveryTestID(text string) ID {
	id, _ := ParseID(text)
	return id
}
