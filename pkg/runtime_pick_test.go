package quint

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"testing"
)

func TestUpstreamRuntimePicks(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Source string
		Input  Expr
		Bounds []Integer
		Picks  []struct {
			Positions []Integer
			Output    *Expr
			Error     *Diagnostic
		}
		Sequences []struct {
			Seed  string
			Draws []struct {
				Position Integer
				Output   Expr
				State    string
			}
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_pick.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Source, func(t *testing.T) {
			ctx := context.Background()
			value, err := EvaluatePureExpression(ctx, fixture.Input, nil)
			if err != nil {
				t.Fatal(err)
			}
			bounds, err := RuntimePickBounds(value)
			if err != nil || len(bounds) != len(fixture.Bounds) || bounds[0] != fixture.Bounds[0] {
				t.Fatal("bounds", bounds, fixture.Bounds, err)
			}
			check := func(got RuntimeValue, expression Expr) {
				t.Helper()
				want, err := NewRuntimeValue(expression)
				if err != nil || !got.Equal(want) {
					t.Fatal("picked value", got.String(), want.String(), err)
				}
			}
			for _, pick := range fixture.Picks {
				got, err := PickRuntimePosition(ctx, value, pick.Positions)
				if pick.Error != nil {
					var diagnostic Diagnostic
					if !errors.As(err, &diagnostic) || diagnostic.Code != pick.Error.Code || diagnostic.Message != pick.Error.Message {
						t.Fatal("pick diagnostic", err, pick.Error)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				check(got, *pick.Output)
			}
			for _, sequence := range fixture.Sequences {
				seed, err := ParseInteger(sequence.Seed)
				if err != nil {
					t.Fatal(err)
				}
				rng := NewRNG(seed)
				for _, draw := range sequence.Draws {
					got, err := PickRuntimeValue(ctx, value, rng)
					if err != nil || rng.State().String() != draw.State {
						t.Fatal("draw", rng.State(), draw.State, err)
					}
					check(got, draw.Output)
					at, err := PickRuntimePosition(ctx, value, []Integer{draw.Position})
					if err != nil || !sameRuntimeObject(got, at) {
						t.Fatal("RNG draw rebuilt stored value", err)
					}
				}
			}
		})
	}
}

func TestRuntimePickingRetainsChildrenAndInputs(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	expression, err := ParseExpression("Set(Map((2,20),(1,10)),Map((3,30)))")
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := runtimeEntries(value)
	before, _ := runtimeMap(value)
	bounds, err := RuntimePickBounds(value)
	if err != nil {
		t.Fatal(err)
	}
	bounds[0] = Integer{}
	fresh, _ := RuntimePickBounds(value)
	if fresh[0].String() != "2" {
		t.Fatal("bounds result aliases internal storage")
	}
	for i, entry := range entries {
		position := IntegerFromBig(big.NewInt(int64(i)))
		positions := []Integer{position, Integer{}}
		got, err := PickRuntimePosition(ctx, value, positions)
		if err != nil || !sameRuntimeObject(got, entry.Value) || positions[0] != position {
			t.Fatal("position lost stored object or mutated caller positions", i, err)
		}
		nestedBefore, _ := runtimeMap(entry.Value)
		nestedAfter, _ := runtimeMap(got)
		if nestedBefore.root != nestedAfter.root {
			t.Fatal("pick rebuilt nested map traversal")
		}
		owned := got.Expression()
		owned.Args = nil
		if !sameRuntimeObject(got, entry.Value) || got.String() == "Map()" {
			t.Fatal("mutating expression copy changed picked value")
		}
	}
	after, _ := runtimeMap(value)
	if before.root != after.root {
		t.Fatal("picking mutated parent set")
	}
}

func TestRuntimePickingValidationAndCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	integer := func(text string) Integer {
		n, err := ParseInteger(text)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}
	set, err := NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "bool", Bool: true}, {Kind: "bool", Bool: false}}})
	if err != nil {
		t.Fatal(err)
	}
	empty, _ := NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{}})
	scalar, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	rng := NewRNG(integer("17"))
	for _, value := range []RuntimeValue{{}, scalar, newRuntimeClosure(0, nil), {identity: "forged", expression: Expr{Kind: "app", Opcode: "Set"}}} {
		if _, err := RuntimePickBounds(value); err == nil {
			t.Fatal("bounds accepted invalid value")
		}
		if _, err := PickRuntimePosition(ctx, value, []Integer{{}}); err == nil {
			t.Fatal("position accepted invalid value")
		}
		if _, err := PickRuntimeValue(ctx, value, rng); err == nil || rng.State().String() != "17" {
			t.Fatal("invalid draw consumed RNG", err, rng.State())
		}
	}
	if _, err := PickRuntimeValue(ctx, set, nil); err == nil {
		t.Fatal("nil RNG accepted")
	}
	if _, err := PickRuntimeValue(ctx, empty, rng); err == nil || rng.State().String() != "17" {
		t.Fatal("empty draw consumed RNG", err, rng.State())
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := PickRuntimePosition(canceled, set, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("position ignored cancellation", err)
	}
	if _, err := PickRuntimeValue(canceled, set, rng); !errors.Is(err, context.Canceled) || rng.State().String() != "17" {
		t.Fatal("canceled draw consumed RNG", err, rng.State())
	}
	during := &pickCancelContext{Context: ctx, remaining: 2}
	if _, err := PickRuntimePosition(during, set, []Integer{integer("1")}); !errors.Is(err, context.Canceled) {
		t.Fatal("traversal ignored cancellation", err)
	}
}

type pickCancelContext struct {
	context.Context
	remaining int
}

func (c *pickCancelContext) Err() error {
	c.remaining--
	if c.remaining < 0 {
		return context.Canceled
	}
	return nil
}
