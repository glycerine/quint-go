package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamDefinitionIDRefresh(t *testing.T) {
	var fixtures []struct {
		Name          string
		Input, Output Declaration
	}
	data, err := os.ReadFile("testdata/parser/refresh_ids.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			out, err := RefreshDefinitionIDs(f.Input, NewIDGenerator(), nil, AnalysisOutput{})
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
