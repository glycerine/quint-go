package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type nondetDiscoveryExpressionFixture struct {
	Name, Source                  string
	Input                         Expr
	Before, After, AfterRecompile []nondetPickSnapshot
	RNG                           string
	Error                         *Diagnostic
}
type nondetDiscoveryModuleStep struct {
	Name          string
	Input         Expr
	Before, After []nondetPickSnapshot
	RNG           string
	Error         *Diagnostic
}

func loadNondetDiscovery(t *testing.T) (expressions []nondetDiscoveryExpressionFixture, module Module, steps []nondetDiscoveryModuleStep) {
	t.Helper()
	var fixture struct {
		Expressions []nondetDiscoveryExpressionFixture
		Module      Module
		Steps       []nondetDiscoveryModuleStep
	}
	data, err := os.ReadFile("testdata/parser/runtime_nondet_discovery.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Expressions, fixture.Module, fixture.Steps
}
func TestUpstreamNondetDiscovery(t *testing.T) {
	t.Setenv("QUINT_RETRY_NONDET_SMALLER_THAN", "100")
	fixtures, _, _ := loadNondetDiscovery(t)
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			ctx := context.Background()
			storage := NewVariableStorageWithMetadata(true)
			rng := NewRNG(Integer{})
			discovery := RuntimeNondetDiscovery{}
			if err := discovery.Prepare(ctx, fixture.Input, storage); err != nil {
				t.Fatal(err)
			}
			assertNondetPicks(t, storage, fixture.Before)
			// Evaluation gets independent storage: automatic discovery must
			// supply its own unexecuted branch names, rather than relying on
			// the explicit helper invocation checked above.
			evaluated := NewVariableStorageWithMetadata(true)
			evaluated.Register("x", "x")
			_, err := EvaluateStateExpressionWithRNG(ctx, fixture.Input, evaluated, rng)
			if fixture.Error != nil {
				var diagnostic Diagnostic
				if !errors.As(err, &diagnostic) || diagnostic.Code != fixture.Error.Code || diagnostic.Message != fixture.Error.Message {
					t.Fatal(err, fixture.Error)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			assertNondetPicks(t, evaluated, fixture.After)
			if rng.State().String() != fixture.RNG {
				t.Fatal("RNG", rng.State().String(), fixture.RNG)
			}
			for name, value := range evaluated.NondetPicks.All() {
				storage.NondetPicks.Set(name, value)
			}
			if err = discovery.Prepare(ctx, fixture.Input, storage); err != nil {
				t.Fatal(err)
			}
			assertNondetPicks(t, storage, fixture.AfterRecompile)
		})
	}
}

func TestUpstreamNondetModuleDiscovery(t *testing.T) {
	t.Setenv("QUINT_RETRY_NONDET_SMALLER_THAN", "100")
	_, module, steps := loadNondetDiscovery(t)
	evaluator, err := NewModuleEvaluator(module, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	discovery := RuntimeNondetDiscovery{}
	prepared := NewVariableStorageWithMetadata(true)
	for index, step := range steps {
		if err = discovery.Prepare(context.Background(), step.Input, prepared); err != nil {
			t.Fatal(err)
		}
		assertNondetPicks(t, prepared, step.Before)
		_, err = evaluator.Evaluate(context.Background(), step.Name, nil)
		if step.Error != nil {
			var diagnostic Diagnostic
			if !errors.As(err, &diagnostic) || diagnostic.Code != step.Error.Code || diagnostic.Message != step.Error.Message {
				t.Fatal(index, err, step.Error)
			}
		} else if err != nil {
			t.Fatal(index, err)
		}
		assertNondetPicks(t, evaluator.Storage, step.After)
		if evaluator.RNG.State().String() != step.RNG {
			t.Fatal(index, "RNG", evaluator.RNG.State().String(), step.RNG)
		}
		for name, value := range evaluator.Storage.NondetPicks.All() {
			prepared.NondetPicks.Set(name, value)
		}
	}
}

func TestNondetDiscoveryCancellationAndShadowing(t *testing.T) {
	t.Parallel()
	discovery := RuntimeNondetDiscovery{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	storage := NewVariableStorage()
	if err := discovery.Prepare(ctx, Expr{}, storage); !errors.Is(err, context.Canceled) {
		t.Fatal("cancel", err)
	}
	if err := discovery.Prepare(context.Background(), Expr{}, nil); err != nil {
		t.Fatal(err)
	}
	malformed := []Expr{{Kind: "let"}, {Kind: "lambda"}, {Kind: "app", Opcode: "assign"}}
	for _, expression := range malformed {
		if err := discovery.Prepare(context.Background(), expression, storage); err != nil {
			t.Fatal(err)
		}
	}
	expression, err := ParseExpression("{ nondet p=Set(1).oneOf(); true }")
	if err != nil {
		t.Fatal(err)
	}
	if err = discovery.Prepare(context.Background(), expression, storage); err != nil {
		t.Fatal(err)
	}
	picked, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	storage.NondetPicks.Set("p", picked)
	if err = discovery.Prepare(context.Background(), expression, storage); err != nil {
		t.Fatal(err)
	}
	preserved, _ := storage.NondetPicks.Get2("p")
	if !sameRuntimeObject(picked, preserved) {
		t.Fatal("recompilation cleared existing pick")
	}
	// A distinct compiled binding of the same name replaces the initial slot.
	expression.Opdef.ID, _ = ParseID("999999")
	if err = discovery.Prepare(context.Background(), expression, storage); err != nil {
		t.Fatal(err)
	}
	cleared, _ := storage.NondetPicks.Get2("p")
	if cleared.Valid() {
		t.Fatal("new binding kept earlier value")
	}
}
