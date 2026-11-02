package quint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUpstreamModules(t *testing.T) {
	paths, _ := filepath.Glob("../evaluator/fixtures/*.json")
	if os.Getenv("QUINT_CORPUS_TESTS") != "1" {
		paths = []string{"../evaluator/fixtures/simple.json"}
	}
	count := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Modules []Module `json:"modules"`
		}
		if err = json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		encoded, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		var got struct {
			Modules []Module `json:"modules"`
		}
		if err = json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(doc, got) {
			t.Fatalf("%s changed on round trip", path)
		}
		for _, m := range doc.Modules {
			count += len(m.Declarations)
		}
	}
	if count == 0 {
		t.Fatal("no declarations tested")
	}
	t.Logf("round-tripped %d declarations", count)
}
func TestModuleDeclarationVariants(t *testing.T) {
	samples := []string{
		`{"id":1,"kind":"const","name":"N","typeAnnotation":{"kind":"int"}}`,
		`{"id":2,"kind":"var","name":"x","typeAnnotation":{"kind":"int"}}`,
		`{"id":3,"kind":"assume","name":"_","assumption":{"id":4,"kind":"bool","value":true}}`,
		`{"id":5,"kind":"typedef","name":"T","params":["a"],"type":{"kind":"var","name":"a"}}`,
		`{"id":6,"kind":"import","protoName":"M","defName":"*","fromSource":"./m","doc":""}`,
		`{"id":7,"kind":"export","protoName":"M","qualifiedName":"Alias"}`,
		`{"id":8,"kind":"instance","protoName":"M","overrides":[[{"id":9,"name":"N"},{"id":10,"kind":"int","value":42}]],"identityOverride":false}`,
	}
	for _, s := range samples {
		var d Declaration
		if err := json.Unmarshal([]byte(s), &d); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(d)
		if err != nil {
			t.Fatal(err)
		}
		var want, got any
		if err = json.Unmarshal([]byte(s), &want); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("changed schema: %s -> %s", s, data)
		}
	}
}
