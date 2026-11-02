package quint

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamModuleImportFlattening(t *testing.T) {
	var fixtures []struct {
		Name   string
		Input  []Module
		Output Module
	}
	data, err := os.ReadFile("testdata/parser/flatten_module.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, _ := ResolveLinkedModules(f.Input)
			out, err := FlattenModuleImports(f.Input[len(f.Input)-1], linked.Table)
			if err != nil {
				t.Fatal(err)
			}
			a, _ := json.Marshal(out)
			b, _ := json.Marshal(f.Output)
			var av, bv any
			json.Unmarshal(a, &av)
			json.Unmarshal(b, &bv)
			if !reflect.DeepEqual(av, bv) {
				t.Fatalf("Go %s\nTS %s", a, b)
			}
		})
	}
}
