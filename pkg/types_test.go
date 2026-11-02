package quint

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestUpstreamTypeTables(t *testing.T) {
	paths, err := filepath.Glob("../evaluator/fixtures/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("missing upstream fixtures")
	}
	count := 0
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc struct {
			Types map[ID]TypeScheme `json:"types"`
		}
		if err = json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for id, s := range doc.Types {
			count++
			encoded, err := json.Marshal(s)
			if err != nil {
				t.Fatalf("%s/%s: %v", path, id, err)
			}
			var got TypeScheme
			if err = json.Unmarshal(encoded, &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(s, got) {
				t.Fatalf("%s/%s changed during round trip", path, id)
			}
		}
	}
	if count == 0 {
		t.Fatal("no type schemes exercised")
	}
	t.Logf("round-tripped %d upstream type schemes", count)
}
func TestTypeValidationAndEmptyArrays(t *testing.T) {
	for _, s := range []string{`{"kind":"mystery"}`, `{"kind":"set"}`, `{"kind":"fun","arg":{"kind":"int"}}`, `{"kind":"rec","fields":{"kind":"row"}}`} {
		var typ Type
		if json.Unmarshal([]byte(s), &typ) == nil {
			t.Fatalf("accepted %s", s)
		}
	}
	unit := UnitType()
	if !unit.IsUnit() {
		t.Fatal("unit not recognized")
	}
	data, err := json.Marshal(unit)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"kind":"tup","fields":{"kind":"row","fields":[],"other":{"kind":"empty"}}}` {
		t.Fatalf("unit JSON: %s", data)
	}
}
