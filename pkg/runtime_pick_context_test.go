package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func TestUpstreamCheckedRuntimeDomainPicks(t *testing.T) {
	t.Parallel()
	var fixtures struct{ Domains []compoundDomainSnapshot }
	data, err := os.ReadFile("testdata/parser/runtime_domain_compound.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Domains {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			value, err := RuntimeValueFromDomain(compoundFixtureDomain(t, fixture.Input))
			if err != nil {
				t.Fatal(err)
			}
			bounds, err := RuntimePickBoundsContext(ctx, value)
			checkDomainOutcome(t, compoundSerializedBounds(bounds), err, fixture.Expected.Bounds)
			serialize := func(value RuntimeValue, err error) (Expr, error) {
				if err != nil {
					return Expr{}, err
				}
				return SerializeRuntimeValueContext(ctx, value, nil)
			}
			for _, pick := range fixture.Picks {
				positions := compoundFixturePositions(t, pick.Positions)
				picked, err := PickRuntimePositionContext(ctx, value, positions)
				expression, err := serialize(picked, err)
				checkDomainOutcome(t, expression, err, pick.domainFixtureOutcome)
				// The existing contextual entry point also accepts retained domains.
				picked, err = PickRuntimePosition(ctx, value, positions)
				expression, err = serialize(picked, err)
				checkDomainOutcome(t, expression, err, pick.domainFixtureOutcome)
			}
			for _, sample := range fixture.Samples {
				rng := NewRNG(domainFixtureInteger(t, sample.Seed))
				legacyRNG := NewRNG(domainFixtureInteger(t, sample.Seed))
				for _, draw := range sample.Draws {
					picked, err := PickRuntimeValueContext(ctx, value, rng)
					expression, err := serialize(picked, err)
					checkDomainOutcome(t, expression, err, draw.domainFixtureOutcome)
					legacy, err := PickRuntimeValue(ctx, value, legacyRNG)
					expression, err = serialize(legacy, err)
					checkDomainOutcome(t, expression, err, draw.domainFixtureOutcome)
					if rng.State().String() != draw.State || legacyRNG.State().String() != draw.State {
						t.Fatal("RNG state", rng.State(), legacyRNG.State(), draw.State)
					}
				}
			}
		})
	}
}

func TestCheckedRuntimePickingValidationCancellationAndIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	expression, err := ParseExpression("Set(Map((2,20),(1,10)),Map((3,30)))")
	if err != nil {
		t.Fatal(err)
	}
	set, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := runtimeEntries(set)
	positions := []Integer{{}}
	picked, err := PickRuntimePositionContext(ctx, set, positions)
	if err != nil || !sameRuntimeObject(picked, entries[0].Value) || positions[0] != (Integer{}) {
		t.Fatal("picked child identity/input mutation", err)
	}
	for _, value := range []RuntimeValue{{}, entries[0].Value} {
		if _, err := RuntimePickBoundsContext(ctx, value); err == nil {
			t.Fatal("invalid bounds accepted")
		}
		if _, err := PickRuntimePositionContext(ctx, value, positions); err == nil {
			t.Fatal("invalid pick accepted")
		}
		if _, err := PickRuntimeValueContext(ctx, value, NewRNG(Integer{})); err == nil {
			t.Fatal("invalid sample accepted")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := RuntimePickBoundsContext(canceled, set); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled bounds", err)
	}
	if _, err := PickRuntimePositionContext(canceled, set, positions); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled pick", err)
	}
	rng := NewRNG(Integer{})
	if _, err := PickRuntimeValueContext(canceled, set, rng); !errors.Is(err, context.Canceled) || rng.State() != (Integer{}) {
		t.Fatal("canceled sample consumed RNG", err, rng.State())
	}
	if _, err := PickRuntimeValueContext(ctx, set, nil); err == nil {
		t.Fatal("nonempty sample accepted nil RNG")
	}
	nullary, err := NewRuntimeProduct(nil)
	if err != nil {
		t.Fatal(err)
	}
	value, err := RuntimeValueFromDomain(nullary)
	if err != nil {
		t.Fatal(err)
	}
	picked, err = PickRuntimeValueContext(ctx, value, nil)
	if err != nil || picked.expression.Opcode != "Tup" || len(picked.expression.Args) != 0 {
		t.Fatal("zero-bound sample required RNG", err, picked.String())
	}
}
