package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type symbolicNondetException struct{ Name, Message string }

type symbolicNondetOutcome struct {
	Output     *Expr
	Error      *Diagnostic
	Thrown     *symbolicNondetException
	Picks      []nondetPickSnapshot
	Pending    Expr
	RNG        string
	CacheEmpty bool
}

type symbolicNondetAttempt struct {
	Invocation int
	Name       string
	Value      *Expr
	Error      *Diagnostic
}

type symbolicNondetFixtures struct {
	Helpers []struct {
		Name, Mode, Seed, Limit string
		Input                   compoundDomainSpec
		Target, Prior           *Expr
		Attempts                []symbolicNondetAttempt
		SetCalls                int
		Outputs                 []symbolicNondetOutcome
	}
	Expressions []struct {
		Name, Source, Seed, Limit string
		Input                     Expr
		Output                    *Expr
		Error                     *Diagnostic
		Thrown                    *symbolicNondetException
		Picks, Pending            []nondetPickSnapshot
		RNG                       string
	}
}

func loadSymbolicNondetFixtures(t *testing.T) symbolicNondetFixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/runtime_symbolic_nondet.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures symbolicNondetFixtures
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func assertSymbolicNondetOutcome(t *testing.T, got RuntimeValue, err error, output *Expr, diagnostic *Diagnostic, exception *symbolicNondetException) {
	t.Helper()
	if exception != nil {
		var thrown RuntimeDomainException
		if !errors.As(err, &thrown) || thrown.Name != exception.Name || thrown.Message != exception.Message {
			t.Fatalf("exception: got %v, want %+v", err, exception)
		}
		return
	}
	assertNondetOutcome(t, got, err, output, diagnostic)
}

func TestUpstreamSymbolicNondetRetries(t *testing.T) {
	t.Parallel()
	fixtures := loadSymbolicNondetFixtures(t)
	for _, fixture := range fixtures.Helpers {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			domain := compoundFixtureDomain(t, fixture.Input)
			set, err := RuntimeValueFromDomain(domain)
			if err != nil {
				t.Fatal(err)
			}
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
			next.Value, _ = NewRuntimeValue(Expr{Kind: "int", Int: Integer{decimal: "-99"}})
			prior, target := RuntimeValue{}, RuntimeValue{}
			if fixture.Prior != nil {
				prior, err = NewRuntimeValue(*fixture.Prior)
				if err != nil {
					t.Fatal(err)
				}
			}
			if fixture.Target != nil {
				target, err = NewRuntimeValue(*fixture.Target)
				if err != nil {
					t.Fatal(err)
				}
			}
			storage.NondetPicks.Set("p", prior)
			var attempts []symbolicNondetAttempt
			setCalls, invocation := 0, 0
			setEval := func(context.Context) (RuntimeValue, error) {
				setCalls++
				return set, nil
			}
			bodyEval := func(ctx context.Context, picked RuntimeValue, pickErr error) (RuntimeValue, error) {
				attempt := symbolicNondetAttempt{Invocation: invocation, Name: "p"}
				if pickErr != nil {
					var diagnostic Diagnostic
					if !errors.As(pickErr, &diagnostic) {
						t.Fatal("exception reached cached body", pickErr)
					}
					attempt.Error = &diagnostic
				} else {
					expression, err := SerializeRuntimeValueContext(ctx, picked, storage.ObjectHashState())
					if err != nil {
						t.Fatal("picked display", err)
					}
					attempt.Value = &expression
				}
				attempts = append(attempts, attempt)
				if len(attempts) > len(fixture.Attempts) {
					t.Fatal("unexpected extra retry")
				}
				truth := func(value bool) (RuntimeValue, error) {
					return NewRuntimeValue(Expr{Kind: "bool", Bool: value})
				}
				if fixture.Mode == "ignoreTrue" || fixture.Mode == "ignoreFalse" {
					return truth(fixture.Mode == "ignoreTrue")
				}
				if pickErr != nil {
					return RuntimeValue{}, pickErr
				}
				if fixture.Mode == "error" {
					return RuntimeValue{}, Diagnostic{Code: "QNT999", Message: "body failure"}
				}
				if fixture.Mode == "assignFalse" || fixture.Mode == "assignTarget" {
					next.Value = picked
				}
				matched := false
				if target.Valid() {
					matched, err = picked.EqualContext(ctx, target)
					if err != nil {
						t.Fatal(err)
					}
				}
				return truth(fixture.Mode == "true" || (fixture.Mode == "target" || fixture.Mode == "assignTarget") && matched)
			}
			for invocation = range fixture.Outputs {
				outcome := fixture.Outputs[invocation]
				got, err := EvalNondetWithOptions(ctx, "p", storage, rng, setEval, bodyEval, RuntimeNondetOptions{RetrySmallerThan: limit})
				assertSymbolicNondetOutcome(t, got, err, outcome.Output, outcome.Error, outcome.Thrown)
				assertNondetPicks(t, storage, outcome.Picks)
				pending, err := NewRuntimeValue(outcome.Pending)
				if err != nil || !next.Value.Equal(pending) || rng.State().String() != outcome.RNG || !outcome.CacheEmpty {
					t.Fatalf("pending=%s want=%s rng=%s want=%s cacheempty=%v error=%v", next.Value.String(), pending.String(), rng.State(), outcome.RNG, outcome.CacheEmpty, err)
				}
			}
			if setCalls != fixture.SetCalls || len(attempts) != len(fixture.Attempts) {
				t.Fatal("call counts", setCalls, fixture.SetCalls, len(attempts), len(fixture.Attempts))
			}
			for i, want := range fixture.Attempts {
				got := attempts[i]
				if got.Name != want.Name || got.Invocation != want.Invocation || (got.Value == nil) != (want.Value == nil) || (got.Error == nil) != (want.Error == nil) {
					t.Fatalf("attempt %d: got %+v want %+v", i, got, want)
				}
				if want.Error != nil {
					if got.Error.Code != want.Error.Code || got.Error.Message != want.Error.Message {
						t.Fatal("cached pick error", got.Error, want.Error)
					}
				} else {
					left, err := NewRuntimeValue(*got.Value)
					if err != nil {
						t.Fatal(err)
					}
					right, err := NewRuntimeValue(*want.Value)
					if err != nil || !left.Equal(right) {
						t.Fatal("attempt value", left.String(), right.String(), err)
					}
				}
			}
		})
	}
}

func TestUpstreamSymbolicNondetExpressions(t *testing.T) {
	t.Parallel()
	ctx := WithRuntimeNondetOptions(context.Background(), RuntimeNondetOptions{RetrySmallerThan: Integer{decimal: "100"}})
	for _, fixture := range loadSymbolicNondetFixtures(t).Expressions {
		t.Run(fixture.Name, func(t *testing.T) {
			storage := NewVariableStorageWithMetadata(true)
			_, next := storage.Register("x", "x")
			seed, err := ParseInteger(fixture.Seed)
			if err != nil {
				t.Fatal(err)
			}
			rng := NewRNG(seed)
			got, err := EvaluateStateExpressionWithRNG(ctx, fixture.Input, storage, rng)
			assertSymbolicNondetOutcome(t, got, err, fixture.Output, fixture.Error, fixture.Thrown)
			assertNondetPicks(t, storage, fixture.Picks)
			if len(fixture.Pending) == 0 {
				if next.Value.Valid() {
					t.Fatal("unexpected pending update", next.Value.String())
				}
			} else {
				want, err := NewRuntimeValue(*fixture.Pending[0].Value)
				if err != nil || !next.Value.Equal(want) {
					t.Fatal("pending update", next.Value.String(), want.String(), err)
				}
			}
			if rng.State().String() != fixture.RNG {
				t.Fatal("RNG state", rng.State(), fixture.RNG)
			}
		})
	}
}
