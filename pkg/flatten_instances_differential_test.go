package quint

import (
	"encoding/json"
	"github.com/glycerine/insdict"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamInstanceFlattening(t *testing.T) {
	var fixtures []struct {
		Name          string
		Input, Output []Module
	}
	data, err := os.ReadFile("testdata/parser/flatten_instances.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, _ := ResolveLinkedModules(f.Input)
			modules := insdict.NewDict[string, Module]()
			for _, m := range f.Input {
				modules.Set(m.Name, m)
			}
			out, err := FlattenInstances(f.Input[1], modules, linked.Table, NewIDGenerator(), nil, AnalysisOutput{})
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
