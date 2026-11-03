package quint

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"os"
	"testing"
)

func TestUpstreamRNGInitialization(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Initializations []struct {
			Fraction float64
			Seed     string
			Draws    []struct{ Bound, Value, State string }
		}
		Histories []struct {
			Seed  string
			Steps []struct{ Op, Argument, Value, State string }
		}
	}
	data, err := os.ReadFile("testdata/parser/rng_initialization.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Initializations) != 10 || len(fixture.Histories) != 9 {
		t.Fatal("incomplete initialization and counter-history fixtures")
	}
	integer := func(text string) Integer {
		t.Helper()
		value, err := ParseInteger(text)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	for _, initialization := range fixture.Initializations {
		t.Run("default-"+initialization.Seed, func(t *testing.T) {
			seed := integer(initialization.Seed)
			if seed.Big().Sign() < 0 || seed.Big().Cmp(big.NewInt(1<<53-1)) >= 0 {
				t.Fatal("upstream default seed outside the promised range", seed)
			}
			// Inject the exact upstream initial state as entropy. System entropy
			// has no common stream with Math.random; both initialize this range.
			entropy := bytes.NewReader(seed.Big().FillBytes(make([]byte, 7)))
			rng, err := newRandomRNG(entropy)
			if err != nil || rng.State() != seed || entropy.Len() != 0 {
				t.Fatal(rng, err, entropy.Len())
			}
			for _, draw := range initialization.Draws {
				value, err := rng.Next(integer(draw.Bound))
				if err != nil || value.String() != draw.Value || rng.State().String() != draw.State {
					t.Fatalf("bound %s: value %s, state %s, want %s/%s: %v", draw.Bound, value, rng.State(), draw.Value, draw.State, err)
				}
			}
		})
	}
	for _, history := range fixture.Histories {
		t.Run("history-"+history.Seed, func(t *testing.T) {
			rng := NewRNG(integer(history.Seed))
			if rng.State().String() != history.Seed {
				t.Fatal("constructor normalized an explicit state", rng.State())
			}
			for i, step := range history.Steps {
				argument := integer(step.Argument)
				switch step.Op {
				case "set":
					rng.SetState(argument)
				case "next":
					value, err := rng.Next(argument)
					if err != nil || value.String() != step.Value {
						t.Fatalf("step %d: got %s, want %s: %v", i, value, step.Value, err)
					}
				default:
					t.Fatal("unknown reference operation", step.Op)
				}
				if rng.State().String() != step.State {
					t.Fatalf("step %d: state %s, want %s", i, rng.State(), step.State)
				}
			}
		})
	}
}

type rngEntropyFailure struct{ failure error }

func (r rngEntropyFailure) Read([]byte) (int, error) { return 0, r.failure }

func TestRandomRNGEntropyBoundaries(t *testing.T) {
	t.Parallel()
	// All-one low 53 bits equal the excluded maximum and must be rejected.
	// The following candidate is the largest allowed initial state.
	maximum := big.NewInt(1<<53 - 1)
	entropy := append(maximum.FillBytes(make([]byte, 7)), new(big.Int).Sub(maximum, big.NewInt(1)).FillBytes(make([]byte, 7))...)
	rng, err := newRandomRNG(bytes.NewReader(entropy))
	if err != nil || rng.State().String() != "9007199254740990" {
		t.Fatal(rng, err)
	}
	for _, source := range []io.Reader{bytes.NewReader(nil), bytes.NewReader(make([]byte, 6)), rngEntropyFailure{io.ErrClosedPipe}} {
		rng, err := newRandomRNG(source)
		if rng != nil || err == nil {
			t.Fatal("entropy failure produced a generator", rng, err)
		}
	}
	_, err = newRandomRNG(rngEntropyFailure{io.ErrClosedPipe})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("entropy error identity lost", err)
	}
	first, err := NewRandomRNG()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewRandomRNG()
	if err != nil {
		t.Fatal(err)
	}
	for _, rng := range []*RNG{first, second} {
		if rng.State().Big().Sign() < 0 || rng.State().Big().Cmp(maximum) >= 0 {
			t.Fatal("system seed outside the default range", rng.State())
		}
	}
	saved := second.State()
	first.SetState(Integer{})
	if second.State() != saved {
		t.Fatal("independent random generators share state")
	}
	// The lower-level explicit constructor and zero value remain deterministic.
	var zero RNG
	if zero.State() != (Integer{}) || NewRNG(Integer{}).State() != (Integer{}) {
		t.Fatal("explicit zero seed changed")
	}
}

func TestREPLExactInitialSeedAndResetSemantics(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"0", "-3", "18446744073709551619", "-18446744073709551619", "1267650600228229401496703205393"} {
		seed, err := ParseInteger(text)
		if err != nil {
			t.Fatal(err)
		}
		session := NewREPLSessionWithSeed(seed)
		if session.Seed() != seed {
			t.Fatal("session constructor normalized seed", session.Seed(), seed)
		}
		// Source execution uses the same owned RNG that the constructor seeds.
		expected := NewRNG(seed)
		bound, _ := ParseInteger("3")
		_, err = expected.Next(bound)
		if err != nil {
			t.Fatal(err)
		}
		result, err := session.Evaluate(context.Background(), "{ nondet p=1.to(3).oneOf(); p==p }")
		if err != nil || result.Value.String() != "true" || session.Seed() != expected.State() {
			t.Fatal("session execution did not use the exact constructor seed", result, session.Seed(), expected.State(), err)
		}
		session.Reset()
		if session.Seed() != expected.State() {
			t.Fatal("clear normalized or rewound current state", session.Seed(), expected.State())
		}
		session.SetSeed(seed)
		expected.SetState(seed)
		if session.Seed() != expected.State() {
			t.Fatal("session reset did not normalize seed", session.Seed(), expected.State())
		}
	}
}
