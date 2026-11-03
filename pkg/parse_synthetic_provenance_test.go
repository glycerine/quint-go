package quint

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestUpstreamSyntheticParserProvenance(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name, Source    string
		LastID          ID
		Modules         json.RawMessage
		Diagnostics     []Diagnostic
		UpstreamSources UpstreamSourceMap
		NextID          ID
	}
	data, err := os.ReadFile("testdata/parser/parser_synthetic_provenance.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 32 {
		t.Fatal("synthetic provenance inventory", len(fixtures))
	}
	decode := func(data []byte) any {
		var result any
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		if err := decoder.Decode(&result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			result, err := parseModulesRecoveringWithIDs(fixture.Name+".qnt", fixture.Source, NewIDGeneratorAfter(fixture.LastID))
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(result.Modules)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(decode(actual), decode(fixture.Modules)) {
				t.Errorf("raw IR\n got %s\nwant %s", actual, fixture.Modules)
			}
			if !reflect.DeepEqual(result.Diagnostics, fixture.Diagnostics) {
				t.Errorf("diagnostics got%v want%v", result.Diagnostics, fixture.Diagnostics)
			}
			if !reflect.DeepEqual(result.UpstreamSources, fixture.UpstreamSources) {
				t.Errorf("raw sources\n got%v\nwant%v", result.UpstreamSources, fixture.UpstreamSources)
			}
			if got := result.IDs.Next(); got != fixture.NextID {
				t.Errorf("next ID got%s want%s", got, fixture.NextID)
			}
			// The public editor map also covers the synthetic destructuring
			// nodes for which the reference listener allocates no raw span.
			var checkIDs func(any)
			checkIDs = func(value any) {
				switch node := value.(type) {
				case map[string]any:
					if number, ok := node["id"].(json.Number); ok {
						id, err := ParseID(number.String())
						if err != nil {
							t.Fatal(err)
						}
						if _, ok := result.Sources[id]; !ok {
							t.Errorf("public source missing for ID%s", id)
						}
					}
					for _, child := range node {
						checkIDs(child)
					}
				case []any:
					for _, child := range node {
						checkIDs(child)
					}
				}
			}
			checkIDs(decode(actual))
			file, err := NewSourceFile(fixture.Name+".qnt", fixture.Source)
			if err != nil {
				t.Fatal(err)
			}
			for id, raw := range fixture.UpstreamSources {
				public := result.Sources[id]
				if public.Source != raw.Source || public.Start != file.positions[raw.Start.Index] {
					t.Errorf("public start for%s: %+v", id, public)
				}
				if raw.End != nil && (public.End == nil || *public.End != file.positions[raw.End.Index+1]) {
					t.Errorf("public end for%s: %+v", id, public)
				}
			}
		})
	}
}
