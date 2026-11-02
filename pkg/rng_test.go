package quint

import (
	"encoding/json"
	"os"
	"testing"
)

func TestUpstreamRNG(t *testing.T) {
	var fixtures []struct {
		Seed, Initial string
		Reset         bool
		Draws         []struct{ Bound, Value, State string }
	}
	data, err := os.ReadFile("testdata/parser/rng.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, f := range fixtures {
		seed, err := ParseInteger(f.Seed)
		if err != nil {
			t.Fatal(err)
		}
		r := NewRNG(seed)
		if f.Reset {
			r.SetState(seed)
		}
		if r.State().String() != f.Initial {
			t.Fatal(f.Seed, r.State(), f.Initial)
		}
		for _, draw := range f.Draws {
			bound, err := ParseInteger(draw.Bound)
			if err != nil {
				t.Fatal(err)
			}
			saved := r.State()
			value, err := r.Next(bound)
			if err != nil || value.String() != draw.Value || r.State().String() != draw.State {
				t.Fatalf("seed %s reset %v bound %s: value %s state %s, want %s %s: %v", f.Seed, f.Reset, draw.Bound, value.String(), r.State().String(), draw.Value, draw.State, err)
			}
			// Constructor restoration retains the exact counter, including negative states.
			restored := NewRNG(saved)
			again, err := restored.Next(bound)
			if err != nil || again != value || restored.State() != r.State() {
				t.Fatal("replay failed")
			}
		}
	}
}

func TestRNGInvalidBoundsAndOwnership(t *testing.T) {
	var r RNG
	for _, text := range []string{"0", "-1", "-100000000000000000000000"} {
		bound, _ := ParseInteger(text)
		if _, err := r.Next(bound); err == nil || r.State().String() != "0" {
			t.Fatal(text, err, r.State())
		}
	}
	seed, _ := ParseInteger("20230404")
	r.SetState(seed)
	for i, want := range []string{"314", "103", "1254", "39968831094728793095337668312741798650168913551481632943502796831606193252535"} {
		bounds := []string{"1000", "2000", "3000", "115792089237316195423570985008687907853269984665640564039457584007913129639936"}
		bound, _ := ParseInteger(bounds[i])
		got, err := r.Next(bound)
		if err != nil || got.String() != want {
			t.Fatal(got, err)
		}
	}
	saved := r.State()
	copy := saved.Big()
	copy.SetInt64(0)
	if r.State() != saved {
		t.Fatal("state alias")
	}
	one, _ := ParseInteger("1")
	before := r.State()
	got, err := r.Next(one)
	if err != nil || got.String() != "0" || r.State() == before {
		t.Fatal("bound one must advance", got, err)
	}
}
