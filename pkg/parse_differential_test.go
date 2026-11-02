package quint

import (
	"bytes"
	"encoding/json"
	"errors"
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

func TestUpstreamParserDiagnostics(t *testing.T) {
	data, err := os.ReadFile("testdata/parser/errors.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []struct {
		Name, Source string
		Errors       []Diagnostic
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Name, func(t *testing.T) {
			_, err := ParseModules(f.Source)
			var got Diagnostic
			if !errors.As(err, &got) {
				t.Fatalf("expected diagnostic: %v", err)
			}
			if len(f.Errors) != 1 {
				t.Fatal("fixture requires accumulated semantic errors, not yet supported")
			}
			want := f.Errors[0]
			if got.Code != want.Code || got.Message != want.Message || !reflect.DeepEqual(got.Data, want.Data) {
				t.Fatalf("Go: %+v\nTS: %+v", got, want)
			}
		})
	}
}
