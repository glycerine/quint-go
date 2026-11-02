package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type nondetPrecedenceCase struct {
	Name, Mode, Seed string
	FirstLoadLimit   *string
	LaterEnv         string
	Input            compoundDomainSpec
	Prior            Expr
	Attempts         []struct {
		Invocation int
		Value      *Expr
		Error      *Diagnostic
	}
	SetCalls int
	Outputs  []struct {
		Output *Expr
		Error  *Diagnostic
		Thrown *symbolicNondetException
		Picks  []nondetPickSnapshot
		RNG    string
	}
}

type nondetModuleLoad struct {
	Env, Limit string
	Absent     bool
	Thrown     *symbolicNondetException
}

type nondetPrecedenceFixtures struct {
	Cases       []nondetPrecedenceCase
	ModuleLoads []nondetModuleLoad
}

func loadNondetPrecedenceFixtures(t *testing.T) nondetPrecedenceFixtures {
	t.Helper()
	data, err := os.ReadFile("testdata/parser/runtime_nondet_precedence.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures nondetPrecedenceFixtures
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	return fixtures
}

func checkNondetPrecedenceCase(t *testing.T, fixture nondetPrecedenceCase, processDefaults bool) {
	t.Helper()
	ctx := context.Background()
	text, present := "", fixture.FirstLoadLimit != nil
	if present {
		text = *fixture.FirstLoadLimit
	}
	options, err := runtimeNondetOptionsFromEnvironment(text, present)
	if err != nil {
		t.Fatal(err)
	}
	if !processDefaults {
		ctx = WithRuntimeNondetOptions(ctx, options)
	}
	set, err := RuntimeValueFromDomain(compoundFixtureDomain(t, fixture.Input))
	if err != nil {
		t.Fatal(err)
	}
	storage := NewVariableStorage()
	prior, err := NewRuntimeValue(fixture.Prior)
	if err != nil {
		t.Fatal(err)
	}
	storage.NondetPicks.Set("p", prior)
	rng := NewRNG(domainFixtureInteger(t, fixture.Seed))
	attempts, setCalls, invocation := 0, 0, 0
	body := func(ctx context.Context, picked RuntimeValue, pickErr error) (RuntimeValue, error) {
		if attempts >= len(fixture.Attempts) {
			t.Fatal("unexpected extra retry")
		}
		want := fixture.Attempts[attempts]
		attempts++
		if want.Invocation != invocation {
			t.Fatal("attempt invocation", want.Invocation, invocation)
		}
		if want.Error != nil {
			assertNondetOutcome(t, picked, pickErr, nil, want.Error)
		} else {
			assertNondetOutcome(t, picked, pickErr, want.Value, nil)
		}
		switch fixture.Mode {
		case "propagate":
			return picked, pickErr
		case "error":
			return RuntimeValue{}, Diagnostic{Code: "QNT999", Message: "body failure"}
		case "throw":
			return RuntimeValue{}, RuntimeDomainException{Name: "Error", Message: "body thrown"}
		case "int":
			return NewRuntimeValue(Expr{Kind: "int", Int: Integer{decimal: "7"}})
		case "tuple":
			return NewRuntimeValue(Expr{Kind: "app", Opcode: "Tup", Args: []Expr{{Kind: "int", Int: Integer{decimal: "7"}}}})
		default:
			return NewRuntimeValue(Expr{Kind: "bool", Bool: fixture.Mode == "true"})
		}
	}
	for invocation = range fixture.Outputs {
		want := fixture.Outputs[invocation]
		got, err := EvalNondet(ctx, "p", storage, rng, func(context.Context) (RuntimeValue, error) {
			setCalls++
			return set, nil
		}, body)
		assertSymbolicNondetOutcome(t, got, err, want.Output, want.Error, want.Thrown)
		assertNondetPicks(t, storage, want.Picks)
		if rng.State().String() != want.RNG {
			t.Fatal("RNG state", rng.State(), want.RNG)
		}
	}
	if attempts != len(fixture.Attempts) || setCalls != fixture.SetCalls {
		t.Fatal("callback counts", attempts, len(fixture.Attempts), setCalls, fixture.SetCalls)
	}
}

func TestUpstreamNondetCallbackPrecedenceAndOptions(t *testing.T) {
	t.Parallel()
	fixtures := loadNondetPrecedenceFixtures(t)
	for _, fixture := range fixtures.Cases {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			checkNondetPrecedenceCase(t, fixture, false)
		})
	}
	for _, fixture := range fixtures.ModuleLoads {
		t.Run("parse/"+fixture.Env, func(t *testing.T) {
			t.Parallel()
			options, err := runtimeNondetOptionsFromEnvironment(fixture.Env, !fixture.Absent)
			if fixture.Thrown != nil {
				assertSymbolicNondetOutcome(t, RuntimeValue{}, err, nil, nil, fixture.Thrown)
			} else if err != nil || options.RetrySmallerThan.String() != fixture.Limit {
				t.Fatal("environment option", options.RetrySmallerThan, fixture.Limit, err)
			}
		})
	}
}

// A process snapshot needs a fresh test process, not a rebuild or a global
// reset hook. Four tiny child runs cover both threshold directions, defaults,
// and a later malformed environment; parsing forms are covered in-process.
func TestRuntimeNondetProcessSnapshot(t *testing.T) {
	t.Parallel()
	fixtures := loadNondetPrecedenceFixtures(t)
	for _, fixture := range fixtures.Cases {
		if !strings.HasPrefix(fixture.Name, "captured_") {
			continue
		}
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			runNondetSnapshotChild(t, fixture.FirstLoadLimit, struct {
				Case nondetPrecedenceCase
			}{Case: fixture})
		})
	}
	t.Run("invalid_initial_environment", func(t *testing.T) {
		t.Parallel()
		for _, fixture := range fixtures.ModuleLoads {
			if fixture.Thrown != nil {
				runNondetSnapshotChild(t, &fixture.Env, struct {
					Load nondetModuleLoad
				}{Load: fixture})
				return
			}
		}
		t.Fatal("missing malformed environment oracle")
	})
}

func runNondetSnapshotChild(t *testing.T, initial *string, request any) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(binary, "-test.run=^TestRuntimeNondetSnapshotChild$")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "QUINT_RETRY_NONDET_SMALLER_THAN=") && !strings.HasPrefix(entry, "QUINT_GO_NONDET_SNAPSHOT=") {
			command.Env = append(command.Env, entry)
		}
	}
	if initial != nil {
		command.Env = append(command.Env, "QUINT_RETRY_NONDET_SMALLER_THAN="+*initial)
	}
	command.Env = append(command.Env, "QUINT_GO_NONDET_SNAPSHOT="+string(data))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("snapshot child: %v\n%s", err, output)
	}
}

func TestRuntimeNondetSnapshotChild(t *testing.T) {
	data := os.Getenv("QUINT_GO_NONDET_SNAPSHOT")
	if data == "" {
		return
	}
	var request struct {
		Case nondetPrecedenceCase
		Load nondetModuleLoad
	}
	if err := json.Unmarshal([]byte(data), &request); err != nil {
		t.Fatal(err)
	}
	if request.Load.Thrown != nil {
		// Invalid initial configuration stays invalid after the environment
		// changes. Explicit context configuration remains available to embedders.
		t.Setenv("QUINT_RETRY_NONDET_SMALLER_THAN", "100")
		_, err := EvalNondet(context.Background(), "p", nil, nil, nil, nil)
		assertSymbolicNondetOutcome(t, RuntimeValue{}, err, nil, nil, request.Load.Thrown)
		ctx := WithRuntimeNondetOptions(context.Background(), RuntimeNondetOptions{})
		set, err := NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "int", Int: Integer{decimal: "1"}}}})
		if err != nil {
			t.Fatal(err)
		}
		value, err := EvalNondet(ctx, "p", NewVariableStorage(), NewRNG(Integer{}), func(context.Context) (RuntimeValue, error) {
			return set, nil
		}, func(context.Context, RuntimeValue, error) (RuntimeValue, error) {
			return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
		})
		if err != nil || value.expression.Kind != "bool" || !value.expression.Bool {
			t.Fatal("explicit context options did not override process default", err, value.String())
		}
		return
	}
	t.Setenv("QUINT_RETRY_NONDET_SMALLER_THAN", request.Case.LaterEnv)
	checkNondetPrecedenceCase(t, request.Case, true)
}

func TestNondetBodyCancellationPreservesPickMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	domain := NewRuntimeInterval(Integer{decimal: "1"}, Integer{decimal: "3"})
	set, err := RuntimeValueFromDomain(domain)
	if err != nil {
		t.Fatal(err)
	}
	prior, err := NewRuntimeValue(Expr{Kind: "int", Int: Integer{decimal: "19"}})
	if err != nil {
		t.Fatal(err)
	}
	storage := NewVariableStorage()
	storage.NondetPicks.Set("p", prior)
	rng := NewRNG(Integer{})
	_, err = EvalNondetWithOptions(ctx, "p", storage, rng, func(context.Context) (RuntimeValue, error) {
		return set, nil
	}, func(context.Context, RuntimeValue, error) (RuntimeValue, error) {
		return RuntimeValue{}, context.Canceled
	}, RuntimeNondetOptions{RetrySmallerThan: Integer{decimal: "100"}})
	got := storage.NondetPicks.Get("p")
	if !errors.Is(err, context.Canceled) || !sameRuntimeObject(got, prior) || rng.State().String() != "1" {
		t.Fatal("canceled body changed metadata/RNG", err, got.String(), rng.State())
	}
}
