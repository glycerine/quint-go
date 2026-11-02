package quint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRejectMalformedDeclarations(t *testing.T) {
	for _, s := range []string{
		`{"kind":"typedef","name":"T"}`,
		`{"id":1,"kind":"const","name":"x","typeAnnotation":{"kind":"int"},"expr":{"id":2,"kind":"bool","value":true}}`,
		`{"id":1,"kind":"instance","protoName":"M","overrides":[]}`,
		`{"id":1,"kind":"typedef","name":"T","doc":null}`,
		`{"id":1,"kind":"export","protoName":"M","fromSource":"x"}`,
	} {
		var d Declaration
		if json.Unmarshal([]byte(s), &d) == nil {
			t.Fatalf("accepted %s", s)
		}
	}
}
func TestPreserveEmptyTypeParameters(t *testing.T) {
	var d Declaration
	if err := json.Unmarshal([]byte(`{"id":1,"kind":"typedef","name":"T","params":[]}`), &d); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"params":[]`) {
		t.Fatalf("lost explicit empty params: %s", b)
	}
}
