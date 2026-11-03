package quint

import (
	"bytes"
	"encoding/json"
	"reflect"
	"runtime"
	"testing"
)

// FuzzIRJSONDecoding complements the Type/Row target with the enclosing wire
// IR. Successful decodes must reach a canonical wire fixed point; rejected
// input must preserve both the receiver's contents and its owned subtrees.
func FuzzIRJSONDecoding(f *testing.F) {
	for _, seed := range []struct {
		kind uint8
		wire string
	}{
		{0, `{"id":1,"kind":"name","name":"x"}`},
		{0, `{"id":2,"kind":"bool","value":false}`},
		{0, `{"id":3,"kind":"int","value":9007199254740993123456789}`},
		{0, `{"id":4,"kind":"str","value":"😀\u0000"}`},
		{0, `{"id":5,"kind":"app","opcode":"Set","args":[]}`},
		{0, `{"id":6,"kind":"lambda","params":[{"id":7,"name":"x"}],"qualifier":"def","expr":{"id":8,"kind":"name","name":"x"}}`},
		{0, `{"id":9,"kind":"let","opdef":{"id":10,"kind":"def","name":"f","qualifier":"val","expr":{"id":11,"kind":"int","value":0}},"expr":{"id":12,"kind":"name","name":"f"}}`},
		{0, `{"id":1,"kind":"lambda","params":[{"id":2,"name":"x"}],"qualifier":"def","expr":{"id":3,"kind":"app","opcode":"f","args":[null]}}`},
		{1, `{"id":1,"kind":"const","name":"N","typeAnnotation":{"kind":"int"}}`},
		{1, `{"id":1,"kind":"var","name":"x","typeAnnotation":{"kind":"list","elem":{"kind":"str"}}}`},
		{1, `{"id":1,"kind":"assume","name":"_","assumption":{"id":2,"kind":"bool","value":true}}`},
		{1, `{"id":2,"kind":"def","name":"f","qualifier":"val","expr":{"id":3,"kind":"bool","value":true},"namespaces":[]}`},
		{1, `{"id":4,"kind":"typedef","name":"T","params":[],"type":{"kind":"rec","fields":{"kind":"empty"}}}`},
		{1, `{"id":5,"kind":"import","protoName":"M","defName":"*","qualifiedName":"Q","fromSource":"m"}`},
		{1, `{"id":6,"kind":"instance","protoName":"M","overrides":[[{"id":7,"name":"N"},{"id":8,"kind":"int","value":2}]],"identityOverride":true}`},
		{1, `{"id":9,"kind":"def","name":"bad","qualifier":"val","expr":{"id":10,"kind":"bool","value":"bad"}}`},
		{2, `{"id":1,"name":"M","declarations":[]}`},
		{2, `{"id":2,"name":"M","declarations":[{"id":3,"kind":"export","protoName":"N"}],"doc":""}`},
		{2, `{"id":4,"name":"M","declarations":[{"id":5,"kind":"var","name":"x","typeAnnotation":{"kind":"int","extra":0}}]}`},
		{3, `{"id":1,"name":"x","kind":"param","depth":0,"shadowing":{"kind":"var","name":"x"},"typeAnnotation":{"kind":"list","elem":{"kind":"int"}}}`},
		{3, `{"id":2,"name":"x","typeAnnotation":{"kind":"set","elem":null}}`},
		{4, `[{"id":1,"name":"N"},{"id":2,"kind":"int","value":3}]`},
		{4, `[{"id":1,"name":"N"},{"id":2,"kind":"lambda","params":[],"qualifier":"def","expr":null}]`},
		{4, `[null,{"id":1,"kind":"bool","value":true}]`},
	} {
		f.Add(seed.kind, []byte(seed.wire))
	}
	f.Fuzz(func(t *testing.T, kind uint8, wire []byte) {
		if len(wire) > 8<<10 {
			t.Skip()
		}
		expression, declaration, module, parameter, override := jsonIRFuzzReceivers()
		switch kind % 5 {
		case 0:
			jsonIRFuzzDecode(t, wire, expression)
		case 1:
			jsonIRFuzzDecode(t, wire, declaration)
		case 2:
			jsonIRFuzzDecode(t, wire, module)
		case 3:
			jsonIRFuzzDecode(t, wire, parameter)
		case 4:
			jsonIRFuzzDecode(t, wire, override)
		}
	})
}

func jsonIRFuzzReceivers() (Expr, Declaration, Module, Parameter, Override) {
	id, _ := ParseID("9007199254740993123456789")
	depth, hidden, doc := 3, true, "original"
	typ := Type{Kind: "fun", Arg: &Type{Kind: "int"}, Res: &Type{Kind: "list", Elem: &Type{Kind: "str"}}}
	parameter := Parameter{ID: id, Name: "original", Depth: &depth, TypeAnnotation: &typ, Shadowing: json.RawMessage(`{"keep":[1,2]}`)}
	parameters := make([]Parameter, 1, 3)
	parameters[0] = parameter
	args := make([]Expr, 1, 3)
	args[0] = Expr{ID: id, Kind: "str", Str: "original"}
	namespaces := make([]string, 1, 3)
	namespaces[0] = "original"
	expression := Expr{ID: id, Kind: "let", runtimeSource: &runtimeSourceIdentity{},
		Opdef: &OperatorDef{ID: id, Kind: "def", Name: "original", Qualifier: Val, Namespaces: namespaces, Doc: &doc,
			Expr: &Expr{ID: id, Kind: "app", Opcode: "original", Args: args}},
		Body: &Expr{ID: id, Kind: "lambda", Qualifier: Def, Params: parameters, Body: &Expr{ID: id, Kind: "name", Name: "original"}}}
	override := Override{Parameter: parameter, Expr: expression}
	overrides := make([]Override, 1, 3)
	overrides[0] = override
	declaration := Declaration{ID: id, Kind: "instance", ProtoName: "Original", Overrides: overrides, IdentityOverride: true,
		Hidden: &hidden, Depth: &depth, Doc: &doc, Namespaces: namespaces, Shadowing: json.RawMessage(`{"keep":[1,2]}`),
		runtimeVariable: &RuntimeVariableBinding{}}
	declarations := make([]Declaration, 1, 3)
	declarations[0] = declaration
	module := Module{ID: id, Name: "Original", Declarations: declarations, Doc: &doc, runtimeSource: &runtimeSourceModule{}}
	return expression, declaration, module, parameter, override
}

func jsonIRFuzzDecode[T any](t *testing.T, wire []byte, original T) {
	t.Helper()
	before, err := json.Marshal(original)
	if err != nil {
		t.Fatal("invalid fuzz receiver", err)
	}
	identities := jsonIRFuzzIdentities(reflect.ValueOf(original))
	value := original
	if err = json.Unmarshal(wire, &value); err != nil {
		after, marshalErr := json.Marshal(value)
		if marshalErr != nil || !bytes.Equal(before, after) || !reflect.DeepEqual(identities, jsonIRFuzzIdentities(reflect.ValueOf(value))) {
			t.Fatalf("rejected %T decode changed receiver: %v\nbefore %s\n after %s", value, err, before, after)
		}
		// Keep the original pointer/slice owners live through the identity check.
		runtime.KeepAlive(original)
		return
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("accepted %T cannot be encoded: %v", value, err)
	}
	var roundtrip T
	if err = json.Unmarshal(canonical, &roundtrip); err != nil {
		t.Fatalf("accepted %T cannot decode its canonical wire: %v\n%s", value, err, canonical)
	}
	second, err := json.Marshal(roundtrip)
	if err != nil || !bytes.Equal(canonical, second) {
		t.Fatalf("%T canonical wire is not idempotent: %v\nfirst %s\n next %s", value, err, canonical, second)
	}
}

type jsonIRFuzzIdentity struct {
	path     string
	kind     reflect.Kind
	address  uintptr
	len, cap int
}

func jsonIRFuzzIdentities(value reflect.Value) []jsonIRFuzzIdentity {
	identities := []jsonIRFuzzIdentity{}
	var walk func(reflect.Value, string, bool)
	walk = func(value reflect.Value, path string, opaque bool) {
		switch value.Kind() {
		case reflect.Pointer:
			identities = append(identities, jsonIRFuzzIdentity{path: path, kind: value.Kind(), address: value.Pointer()})
			if !value.IsNil() && !opaque {
				walk(value.Elem(), path+"*", false)
			}
		case reflect.Slice:
			identities = append(identities, jsonIRFuzzIdentity{path: path, kind: value.Kind(), address: value.Pointer(), len: value.Len(), cap: value.Cap()})
			for i := 0; i < value.Len(); i++ {
				walk(value.Index(i), path+"[]", false)
			}
		case reflect.Struct:
			for i := 0; i < value.NumField(); i++ {
				field := value.Type().Field(i)
				walk(value.Field(i), path+"."+field.Name, field.PkgPath != "")
			}
		}
	}
	walk(value, "", false)
	return identities
}
