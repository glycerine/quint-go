package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamDefinitionNamespace(t *testing.T) {
	var fixtures []struct {
		Name, Namespace string
		Input, Output   Declaration
		Preserve        []string
	}
	data, err := os.ReadFile("testdata/parser/namespace.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			preserve := map[string]bool{}
			for _, n := range f.Preserve {
				preserve[n] = true
			}
			out, err := AddNamespaceToDefinition(f.Input, f.Namespace, preserve)
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
