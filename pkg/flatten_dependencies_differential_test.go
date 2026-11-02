package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamFlattenDependencies(t *testing.T) {
	var fixtures []struct {
		Name       string
		Input      []Module
		Expression Expr
		Output     []Declaration
	}
	data, err := os.ReadFile("testdata/parser/flatten_dependencies.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			linked, _ := ResolveLinkedModules(f.Input)
			out, err := DependentDefinitions(f.Expression, linked.Table)
			if err != nil {
				t.Fatal(err)
			}

			a, _ := json.Marshal(out)
			b, _ := json.Marshal(f.Output)
			if string(a) != string(b) {
				t.Fatalf("Go %s\nTS %s", a, b)
			}
		})
	}
}
