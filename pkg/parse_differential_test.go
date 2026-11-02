package quint

import (
	"bytes"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Only numeric IDs are normalized: Go allocates identities in a different order
// and does not yet attach type IDs. All other IR fields must match upstream.
func withoutParserIDs(v any) any {
	switch x := v.(type) {
	case map[string]any:
		delete(x, "id")
		for k, value := range x {
			x[k] = withoutParserIDs(value)
		}
	case []any:
		for i, value := range x {
			x[i] = withoutParserIDs(value)
		}
	}
	return v
}
func TestUpstreamPhaseOneFixtures(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/phase1.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name, Source string
		Modules      json.RawMessage
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	decode := func(data []byte) any {
		t.Helper()
		var v any
		d := json.NewDecoder(bytes.NewReader(data))
		d.UseNumber()
		if err := d.Decode(&v); err != nil {
			t.Fatal(err)
		}
		return withoutParserIDs(v)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			ms, err := ParseModules(f.Source)
			if err != nil {
				t.Fatal(err)
			}
			actual, err := json.Marshal(ms)
			if err != nil {
				t.Fatal(err)
			}
			got, want := decode(actual), decode(f.Modules)
			if !reflect.DeepEqual(got, want) {
				g, _ := json.Marshal(got)
				w, _ := json.Marshal(want)
				t.Fatalf("Go: %s\nTS: %s", g, w)
			}
		})
	}
}
