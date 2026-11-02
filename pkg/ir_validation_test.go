package quint

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMalformedIRLeavesReceiverUnchanged(t *testing.T) {
	tests := []struct {
		name   string
		target any
		inputs []string
	}{
		{"module", &Module{Name: "original"}, []string{`null`, `[]`, `{}`, `{"id":1,"name":"","declarations":[]}`, `{"id":"bad","name":"M","declarations":[]}`, `{"id":1,"name":"M","declarations":null}`}},
		{"parameter", &Parameter{Name: "original"}, []string{`null`, `{}`, `{"id":1,"name":""}`, `{"id":1,"name":false}`, `{"id":1,"name":"p","extra":1}`}},
		{"definition", &OperatorDef{Name: "original"}, []string{`null`, `{}`, `{"id":1,"kind":"typedef","name":"T"}`, `{"id":1,"kind":"def","name":"f","qualifier":"invalid","expr":{"id":2,"kind":"bool","value":true}}`}},
		{"override", &Override{}, []string{`null`, `[]`, `[{}]`, `[{},{}]`, `[{"id":1,"name":"x"},null]`, `[{"id":1,"name":"x"},{"id":2,"kind":"int","value":1},{}]`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := reflect.ValueOf(tt.target).Elem().Interface()
			for _, s := range tt.inputs {
				if err := json.Unmarshal([]byte(s), tt.target); err == nil {
					t.Errorf("accepted %s", s)
				}
				if !reflect.DeepEqual(before, reflect.ValueOf(tt.target).Elem().Interface()) {
					t.Fatalf("failed decode mutated receiver for %s", s)
				}
			}
		})
	}
}

func TestAllTypeVariants(t *testing.T) {
	samples := []string{
		`{"kind":"bool"}`, `{"kind":"int"}`, `{"kind":"str"}`, `{"kind":"const","name":"T"}`, `{"kind":"var","name":"a"}`,
		`{"kind":"set","elem":{"kind":"int"}}`, `{"kind":"list","elem":{"kind":"str"}}`,
		`{"kind":"fun","arg":{"kind":"int"},"res":{"kind":"bool"}}`,
		`{"kind":"oper","args":[],"res":{"kind":"bool"}}`,
		`{"kind":"app","ctor":{"kind":"const","name":"Box"},"args":[{"kind":"int"}]}`,
		`{"kind":"rec","fields":{"kind":"var","name":"r"}}`,
		`{"kind":"sum","fields":{"kind":"row","fields":[{"fieldName":"Some","fieldType":{"kind":"int"}}],"other":{"kind":"empty"}}}`,
		`{"kind":"tup","fields":{"kind":"row","fields":[],"other":{"kind":"empty"}}}`,
	}
	for _, s := range samples {
		var typ Type
		if err := json.Unmarshal([]byte(s), &typ); err != nil {
			t.Fatal(err)
		}
		b, err := json.Marshal(typ)
		if err != nil {
			t.Fatal(err)
		}
		var want, got any
		if err = json.Unmarshal([]byte(s), &want); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(b, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("schema changed: %s -> %s", s, b)
		}
	}
}
func TestInvalidTypeChildren(t *testing.T) {
	for _, s := range []string{`null`, `{"kind":"const"}`, `{"kind":"var"}`, `{"kind":"list"}`, `{"kind":"oper","args":[]}`, `{"kind":"app","ctor":{"kind":"int"},"args":[]}`, `{"kind":"sum","fields":{"kind":"var","name":"r"}}`, `{"kind":"rec","fields":{"kind":"var"}}`, `{"kind":"rec","fields":{"kind":"unknown"}}`} {
		var typ Type
		if json.Unmarshal([]byte(s), &typ) == nil {
			t.Errorf("accepted %s", s)
		}
	}
}
