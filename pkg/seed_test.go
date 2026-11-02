package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamSeedParsing(t *testing.T) {
	var fixtures []struct{ Input, Value, Error string }
	data, err := os.ReadFile("testdata/parser/seed.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		t.Run(f.Input, func(t *testing.T) {
			got, err := ParseSeed(f.Input)
			if f.Error != "" {
				if err == nil || err.Error() != f.Error {
					t.Fatalf("%q: %v, want %q", f.Input, err, f.Error)
				}
				return
			}
			if err != nil || got.String() != f.Value {
				t.Fatalf("%q: %s %v, want %s", f.Input, got.String(), err, f.Value)
			}
			// Parsed decimal and prefixed forms feed the same exact seeded stream.
			want, _ := ParseInteger(f.Value)
			r := NewRNG(got)
			reference := NewRNG(want)
			bound, _ := ParseInteger("1000")
			actual, err := r.Next(bound)
			expected, _ := reference.Next(bound)
			if err != nil || actual != expected || r.State() != reference.State() {
				t.Fatal("parsed seed changed stream")
			}
		})
	}
}
