package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type nondetPickSnapshot struct {
	Name  string
	Value *Expr
}
type nondetFixtureOutcome struct {
	Output     *Expr
	Error      *Diagnostic
	Picks      []nondetPickSnapshot
	Pending    Expr
	RNG        string
	CacheEmpty bool
}
type nondetFixture struct {
	Name, Mode, Seed, Limit string
	Set                     Expr
	Target                  Integer
	Prior                   *Integer
	Repeat                  int
	Attempts                []struct {
		Name  string
		Value Expr
	}
	SetCalls int
	Outputs  []nondetFixtureOutcome
}
type nondetExpressionFixture struct {
	Source         string
	Input          Expr
	Output         *Expr
	Error          *Diagnostic
	Picks, Pending []nondetPickSnapshot
	RNG            string
}

func loadNondetFixtures(t *testing.T) (helpers []nondetFixture, expressions []nondetExpressionFixture) {
	t.Helper()
	var fixtures struct {
		Helpers     []nondetFixture
		Expressions []nondetExpressionFixture
	}
	data, err := os.ReadFile("testdata/parser/runtime_nondet.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures.Helpers, fixtures.Expressions
}
func assertNondetPicks(t *testing.T, storage *VariableStorage, want []nondetPickSnapshot) {
	t.Helper()
	index := 0
	for name, value := range storage.NondetPicks.All() {
		if index >= len(want) || name != want[index].Name {
			t.Fatal("pick order", index, name, want)
		}
		if want[index].Value == nil {
			if value.Valid() {
				t.Fatal("expected unset pick", name)
			}
		} else {
			expected, err := NewRuntimeValue(*want[index].Value)
			if err != nil || !expected.Equal(value) {
				t.Fatal("pick", name, value.String(), expected.String(), err)
			}
		}
		index++
	}
	if index != len(want) {
		t.Fatal("missing picks", index, len(want))
	}
}
func assertNondetOutcome(t *testing.T, got RuntimeValue, err error, output *Expr, diagnostic *Diagnostic) {
	t.Helper()
	if diagnostic != nil {
		var gotDiagnostic Diagnostic
		if !errors.As(err, &gotDiagnostic) || gotDiagnostic.Code != diagnostic.Code || gotDiagnostic.Message != diagnostic.Message {
			t.Fatal("diagnostic", err, diagnostic)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	expected, err := NewRuntimeValue(*output)
	if err != nil || !got.Equal(expected) {
		t.Fatal("outcome", got.String(), expected.String(), err)
	}
}

func TestUpstreamNondetRetries(t *testing.T) {
	t.Parallel()
	fixtures, _ := loadNondetFixtures(t)
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			ctx := context.Background()
			seed, err := ParseInteger(fixture.Seed)
			if err != nil {
				t.Fatal(err)
			}
			limit, err := ParseSeed(fixture.Limit)
			if err != nil {
				t.Fatal(err)
			}
			rng := NewRNG(seed)
			storage := NewVariableStorageWithMetadata(true)
			_, next := storage.Register("x", "x")
			minus99, _ := ParseInteger("-99")
			next.Value, _ = NewRuntimeValue(Expr{Kind: "int", Int: minus99})
			prior := RuntimeValue{}
			if fixture.Prior != nil {
				prior, _ = NewRuntimeValue(Expr{Kind: "int", Int: *fixture.Prior})
			}
			storage.NondetPicks.Set("p", prior)
			nested := fixture.Mode == "nested" || fixture.Mode == "nestedFalse"
			if nested {
				storage.NondetPicks.Set("q", RuntimeValue{})
			}
			set, err := NewRuntimeValue(fixture.Set)
			if err != nil {
				t.Fatal(err)
			}
			var attempts []struct {
				Name  string
				Value RuntimeValue
			}
			setCalls := 0
			setEval := func(context.Context) (RuntimeValue, error) {
				setCalls++
				if fixture.Mode == "setError" {
					return RuntimeValue{}, Diagnostic{Code: "QNT999", Message: "set failure"}
				}
				return set, nil
			}
			truth := func(value bool) (RuntimeValue, error) { return NewRuntimeValue(Expr{Kind: "bool", Bool: value}) }
			bodyEval := func(ctx context.Context, picked RuntimeValue, pickErr error) (RuntimeValue, error) {
				if pickErr != nil {
					return RuntimeValue{}, pickErr
				}
				attempts = append(attempts, struct {
					Name  string
					Value RuntimeValue
				}{"p", picked})
				target := picked.expression.Int.Big().Cmp(fixture.Target.Big()) == 0
				if fixture.Mode == "error" || fixture.Mode == "errorTarget" && target {
					return RuntimeValue{}, Diagnostic{Code: "QNT999", Message: "body failure"}
				}
				if fixture.Mode == "assignFalse" || fixture.Mode == "assignTarget" {
					next.Value = picked
				}
				if nested {
					innerSet, err := NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "int", Int: Integer{decimal: "4"}}, {Kind: "int", Int: Integer{decimal: "5"}}, {Kind: "int", Int: Integer{decimal: "6"}}}})
					if err != nil {
						t.Fatal(err)
					}
					_, err = EvalNondetWithOptions(ctx, "q", storage, rng, func(context.Context) (RuntimeValue, error) { return innerSet, nil }, func(ctx context.Context, picked RuntimeValue, pickErr error) (RuntimeValue, error) {
						if pickErr != nil {
							return RuntimeValue{}, pickErr
						}
						attempts = append(attempts, struct {
							Name  string
							Value RuntimeValue
						}{"q", picked})
						return truth(picked.expression.Int.String() == "5")
					}, RuntimeNondetOptions{RetrySmallerThan: limit})
					if err != nil {
						return RuntimeValue{}, err
					}
					return truth(fixture.Mode == "nested" && target)
				}
				return truth(fixture.Mode == "true" || (fixture.Mode == "target" || fixture.Mode == "assignTarget") && target)
			}
			for _, outcome := range fixture.Outputs {
				got, err := EvalNondetWithOptions(ctx, "p", storage, rng, setEval, bodyEval, RuntimeNondetOptions{RetrySmallerThan: limit})
				assertNondetOutcome(t, got, err, outcome.Output, outcome.Error)
				assertNondetPicks(t, storage, outcome.Picks)
				expected, err := NewRuntimeValue(outcome.Pending)
				if err != nil || !next.Value.Equal(expected) || rng.State().String() != outcome.RNG || !outcome.CacheEmpty {
					t.Fatal("pending/RNG/cache", next.Value.String(), expected.String(), rng.State().String(), outcome.RNG, err)
				}
			}
			if setCalls != fixture.SetCalls || len(attempts) != len(fixture.Attempts) {
				t.Fatal("calls", setCalls, fixture.SetCalls, len(attempts), len(fixture.Attempts))
			}
			for i, want := range fixture.Attempts {
				expected, err := NewRuntimeValue(want.Value)
				if err != nil || attempts[i].Name != want.Name || !attempts[i].Value.Equal(expected) {
					t.Fatal("attempt", i, attempts[i].Name, want.Name, attempts[i].Value.String(), expected.String(), err)
				}
			}
		})
	}
}

func TestUpstreamNondetExpressions(t *testing.T) {
	// The reference fixture uses its default limit; explicit retry tests above
	// exercise overrides without modifying process-global environment in parallel.
	t.Setenv("QUINT_RETRY_NONDET_SMALLER_THAN", "100")
	_, fixtures := loadNondetFixtures(t)
	for _, fixture := range fixtures {
		t.Run(fixture.Source, func(t *testing.T) {
			storage := NewVariableStorageWithMetadata(true)
			_, next := storage.Register("x", "x")
			rng := NewRNG(Integer{})
			got, err := EvaluateStateExpressionWithRNG(context.Background(), fixture.Input, storage, rng)
			assertNondetOutcome(t, got, err, fixture.Output, fixture.Error)
			assertNondetPicks(t, storage, fixture.Picks)
			if len(fixture.Pending) == 0 {
				if next.Value.Valid() {
					t.Fatal("unexpected pending value", next.Value.String())
				}
			} else {
				expected, err := NewRuntimeValue(*fixture.Pending[0].Value)
				if err != nil || !next.Value.Equal(expected) {
					t.Fatal("pending", next.Value.String(), expected.String(), err)
				}
			}
			if rng.State().String() != fixture.RNG {
				t.Fatal("RNG", rng.State().String(), fixture.RNG)
			}
		})
	}
}

func TestNondetValidationCancellationAndIdentity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	storage := NewVariableStorage()
	rng := NewRNG(Integer{})
	set, _ := NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "bool", Bool: true}}})
	setEval := func(context.Context) (RuntimeValue, error) { return set, nil }
	bodyEval := func(context.Context, RuntimeValue, error) (RuntimeValue, error) {
		return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	}
	options := RuntimeNondetOptions{RetrySmallerThan: Integer{decimal: "100"}}
	for _, test := range []struct {
		storage  *VariableStorage
		rng      *RNG
		setEval  func(context.Context) (RuntimeValue, error)
		bodyEval func(context.Context, RuntimeValue, error) (RuntimeValue, error)
	}{
		{nil, rng, setEval, bodyEval}, {storage, nil, setEval, bodyEval}, {storage, rng, nil, bodyEval}, {storage, rng, setEval, nil},
		{storage, rng, func(context.Context) (RuntimeValue, error) { return RuntimeValue{}, nil }, bodyEval},
		{storage, rng, func(context.Context) (RuntimeValue, error) { return NewRuntimeValue(Expr{Kind: "bool", Bool: true}) }, bodyEval},
		{storage, rng, setEval, func(context.Context, RuntimeValue, error) (RuntimeValue, error) {
			return NewRuntimeValue(Expr{Kind: "int"})
		}},
	} {
		if _, err := EvalNondetWithOptions(ctx, "p", test.storage, test.rng, test.setEval, test.bodyEval, options); err == nil {
			t.Fatal("accepted invalid invocation")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	calls := 0
	if _, err := EvalNondetWithOptions(canceled, "p", storage, rng, func(context.Context) (RuntimeValue, error) { calls++; return set, nil }, bodyEval, options); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal("pre-cancellation", err, calls)
	}
	canceled, cancel = context.WithCancel(ctx)
	set, _ = NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "bool", Bool: true}, {Kind: "bool", Bool: false}}})
	calls = 0
	if _, err := EvalNondetWithOptions(canceled, "p", storage, rng, setEval, func(context.Context, RuntimeValue, error) (RuntimeValue, error) {
		calls++
		cancel()
		return NewRuntimeValue(Expr{Kind: "bool", Bool: false})
	}, options); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal("retry cancellation", err, calls)
	}
	entries, _ := runtimeEntries(set)
	var picked RuntimeValue
	got, err := EvalNondetWithOptions(ctx, "identity", storage, rng, setEval, func(_ context.Context, value RuntimeValue, pickErr error) (RuntimeValue, error) {
		if pickErr != nil {
			return RuntimeValue{}, pickErr
		}
		picked = value
		if !sameRuntimeObject(value, entries[0].Value) && !sameRuntimeObject(value, entries[1].Value) {
			t.Fatal("rebuilt pick")
		}
		return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	}, options)
	metadata, _ := storage.NondetPicks.Get2("identity")
	if err != nil || !got.expression.Bool || !sameRuntimeObject(metadata, picked) {
		t.Fatal("metadata rebuilt pick", err)
	}
	positions := []Integer{{decimal: "1"}, {decimal: "2"}}
	bounds := []Integer{{decimal: "2"}, {decimal: "3"}}
	incrementRuntimePositions(positions, bounds)
	if positions[0].String() != "0" || positions[1].String() != "0" {
		t.Fatal("increment carry", positions)
	}
	incrementRuntimePositions(positions, bounds)
	if positions[0].String() != "0" || positions[1].String() != "1" {
		t.Fatal("increment", positions)
	}
}

func TestNondetEnvironmentOverride(t *testing.T) {
	t.Setenv("QUINT_RETRY_NONDET_SMALLER_THAN", "bad")
	if _, err := EvalNondet(context.Background(), "p", NewVariableStorage(), NewRNG(Integer{}), nil, nil); err == nil {
		t.Fatal("invalid environment accepted")
	}
	t.Setenv("QUINT_RETRY_NONDET_SMALLER_THAN", "1")
	set, _ := NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "bool", Bool: true}, {Kind: "bool", Bool: false}}})
	calls := 0
	value, err := EvalNondet(context.Background(), "p", NewVariableStorage(), NewRNG(Integer{}), func(context.Context) (RuntimeValue, error) { return set, nil }, func(context.Context, RuntimeValue, error) (RuntimeValue, error) {
		calls++
		return NewRuntimeValue(Expr{Kind: "bool", Bool: false})
	})
	if err != nil || value.expression.Bool || calls != 1 {
		t.Fatal("environment retry override", value.String(), err, calls)
	}
}
