package quint

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"reflect"
	"testing"
)

type primitiveDomainSpec struct {
	Kind, Value, Name, First, Last string
	Args                           []primitiveDomainSpec
}

func domainFixtureInteger(t *testing.T, text string) Integer {
	t.Helper()
	n, err := ParseInteger(text)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func domainFixtureValue(t *testing.T, spec primitiveDomainSpec) RuntimeValue {
	t.Helper()
	var expression Expr
	switch spec.Kind {
	case "int":
		expression = Expr{Kind: "int", Int: domainFixtureInteger(t, spec.Value)}
	case "str":
		expression = Expr{Kind: "str", Str: spec.Value}
	case "bool":
		expression = Expr{Kind: "bool", Bool: spec.Value == "true"}
	case "Set":
		children := make([]RuntimeValue, len(spec.Args))
		for i, child := range spec.Args {
			children[i] = domainFixtureValue(t, child)
		}
		value, err := newRuntimeComposite("Set", children)
		if err != nil {
			t.Fatal(err)
		}
		return value
	default:
		t.Fatal("unsupported primitive value", spec.Kind)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func domainFixtureDomain(t *testing.T, spec primitiveDomainSpec) RuntimeDomain {
	t.Helper()
	switch spec.Kind {
	case "Interval":
		return NewRuntimeInterval(domainFixtureInteger(t, spec.First), domainFixtureInteger(t, spec.Last))
	case "InfSet":
		value, err := NewRuntimeInfiniteSet(spec.Name)
		if err != nil {
			t.Fatal(err)
		}
		return value
	case "Set":
		value, err := RuntimeDomainFromValue(domainFixtureValue(t, spec))
		if err != nil {
			t.Fatal(err)
		}
		return value
	default:
		t.Fatal("unsupported primitive domain", spec.Kind)
		return RuntimeDomain{}
	}
}

// A spec's bool is decoded separately because most value fields are decimal
// strings but Boolean inputs are actual JSON Booleans in the upstream corpus.
func (s *primitiveDomainSpec) UnmarshalJSON(data []byte) error {
	type plain primitiveDomainSpec
	var input struct {
		Kind, Name, First, Last string
		Value                   json.RawMessage
		Args                    []primitiveDomainSpec
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}
	result := primitiveDomainSpec{Kind: input.Kind, Name: input.Name, First: input.First, Last: input.Last, Args: input.Args}
	if len(input.Value) > 0 {
		if input.Kind == "bool" {
			result.Value = string(input.Value)
		} else if err := json.Unmarshal(input.Value, &result.Value); err != nil {
			return err
		}
	}
	*s = result
	return nil
}

type domainFixtureOutcome[T any] struct {
	Value      *T
	Diagnostic *Diagnostic
	Thrown     *RuntimeDomainException
}

func checkDomainOutcome[T any](t *testing.T, got T, err error, want domainFixtureOutcome[T]) {
	t.Helper()
	if want.Diagnostic != nil {
		var diagnostic Diagnostic
		if !errors.As(err, &diagnostic) || diagnostic.Code != want.Diagnostic.Code || diagnostic.Message != want.Diagnostic.Message {
			t.Fatal("diagnostic", err, want.Diagnostic)
		}
		return
	}
	if want.Thrown != nil {
		var exception RuntimeDomainException
		if !errors.As(err, &exception) || exception != *want.Thrown {
			t.Fatal("reference exception", err, want.Thrown)
		}
		return
	}
	if err != nil || want.Value == nil || !reflect.DeepEqual(got, *want.Value) {
		t.Fatalf("outcome got %#v / %v; want %#v", got, err, want.Value)
	}
}

func TestUpstreamPrimitiveRuntimeDomains(t *testing.T) {
	t.Parallel()
	var fixtures []struct {
		Name            string
		Input           primitiveDomainSpec
		SkipEnumeration bool
		Expected        struct {
			HashCode      domainFixtureOutcome[float64]
			ImmutableHash domainFixtureOutcome[int64]
			Cardinality   domainFixtureOutcome[Integer]
			Bounds        domainFixtureOutcome[[]*Integer]
			Display       domainFixtureOutcome[Expr]
			Iteration     domainFixtureOutcome[[]Expr]
			NormalForm    domainFixtureOutcome[struct {
				Class     string
				Value     Expr
				HashCode  int64
				Iteration []Expr
			}]
		}
		Contains, Subset, Equals []struct {
			Input primitiveDomainSpec
			domainFixtureOutcome[bool]
		}
		Picks []struct {
			Positions []string
			domainFixtureOutcome[Expr]
		}
		Samples []struct {
			Seed  string
			Draws []struct {
				Position Integer
				State    string
				domainFixtureOutcome[Expr]
			}
		}
	}
	data, err := os.ReadFile("testdata/parser/runtime_domain.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			ctx := context.Background()
			domain := domainFixtureDomain(t, fixture.Input)
			hash, err := domain.referenceHashCode()
			checkDomainOutcome(t, hash, err, fixture.Expected.HashCode)
			checkDomainOutcome(t, int64(runtimeReferenceSMI(uint32(hash))), err, fixture.Expected.ImmutableHash)
			size, err := domain.Cardinality(ctx)
			checkDomainOutcome(t, size, err, fixture.Expected.Cardinality)
			bounds, err := domain.Bounds(ctx)
			serializedBounds := make([]*Integer, len(bounds))
			for i, bound := range bounds {
				if bound.Finite {
					n := bound.Value
					serializedBounds[i] = &n
				}
			}
			checkDomainOutcome(t, serializedBounds, err, fixture.Expected.Bounds)
			if !fixture.SkipEnumeration {
				expression, err := domain.Expression(ctx)
				checkDomainOutcome(t, expression, err, fixture.Expected.Display)
				values := []Expr{}
				err = domain.ForEach(ctx, func(value RuntimeValue) (bool, error) {
					values = append(values, value.Expression())
					return true, nil
				})
				checkDomainOutcome(t, values, err, fixture.Expected.Iteration)
				normal, err := domain.Materialize(ctx)
				if fixture.Expected.NormalForm.Thrown != nil {
					checkDomainOutcome(t, struct{}{}, err, domainFixtureOutcome[struct{}]{Thrown: fixture.Expected.NormalForm.Thrown})
				} else {
					if err != nil || fixture.Expected.NormalForm.Value == nil {
						t.Fatal("normal form", err)
					}
					want := fixture.Expected.NormalForm.Value
					if !reflect.DeepEqual(normal.Expression(), want.Value) {
						t.Fatal("normal form display", normal.String(), ExpressionString(want.Value))
					}
					hash, err := runtimeReferenceHashCode(normal.expression)
					if err != nil || hash != want.HashCode {
						t.Fatal("normal form hash", hash, want.HashCode, err)
					}
					entries, _ := runtimeEntries(normal)
					values := []Expr{}
					for _, entry := range entries {
						values = append(values, entry.Value.Expression())
					}
					if !reflect.DeepEqual(values, want.Iteration) {
						t.Fatal("normal form traversal", values, want.Iteration)
					}
				}
			}
			for _, query := range fixture.Contains {
				got, err := domain.Contains(ctx, domainFixtureValue(t, query.Input))
				checkDomainOutcome(t, got, err, query.domainFixtureOutcome)
			}
			for _, query := range fixture.Subset {
				got, err := domain.Subset(ctx, domainFixtureDomain(t, query.Input))
				checkDomainOutcome(t, got, err, query.domainFixtureOutcome)
			}
			for _, query := range fixture.Equals {
				got, err := domain.Equal(ctx, domainFixtureDomain(t, query.Input))
				checkDomainOutcome(t, got, err, query.domainFixtureOutcome)
			}
			for _, pick := range fixture.Picks {
				positions := make([]Integer, len(pick.Positions))
				for i, position := range pick.Positions {
					positions[i] = domainFixtureInteger(t, position)
				}
				value, err := domain.Pick(ctx, positions)
				checkDomainOutcome(t, value.Expression(), err, pick.domainFixtureOutcome)
			}
			for _, sample := range fixture.Samples {
				rng := NewRNG(domainFixtureInteger(t, sample.Seed))
				for _, draw := range sample.Draws {
					value, err := domain.Sample(ctx, rng)
					checkDomainOutcome(t, value.Expression(), err, draw.domainFixtureOutcome)
					if rng.State().String() != draw.State {
						t.Fatal("sample RNG", rng.State(), draw.State)
					}
					at, pickErr := domain.Pick(ctx, []Integer{draw.Position})
					if (err == nil) != (pickErr == nil) || (err == nil && !value.Equal(at)) {
						t.Fatal("sample position", value.String(), at.String(), err, pickErr)
					}
				}
			}
		})
	}
}

func TestRuntimeDomainExplicitOwnershipAndEquality(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	expression, err := ParseExpression("Set(Map((2,20),(1,10)),Map((4,40)))")
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	domain, err := RuntimeDomainFromValue(value)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := runtimeMap(value)
	normal, err := domain.Materialize(ctx)
	after, _ := runtimeMap(normal)
	if err != nil || before.root != after.root || sameRuntimeObject(value, normal) {
		t.Fatal("explicit normalization", err)
	}
	entries, _ := runtimeEntries(value)
	calls := 0
	if err := domain.ForEach(ctx, func(got RuntimeValue) (bool, error) {
		if !sameRuntimeObject(got, entries[calls].Value) {
			t.Fatal("iteration rebuilt stored child")
		}
		calls++
		return true, nil
	}); err != nil || calls != 2 {
		t.Fatal("explicit iteration", err, calls)
	}
	picked, err := domain.Pick(ctx, []Integer{{}})
	if err != nil || !sameRuntimeObject(picked, entries[0].Value) {
		t.Fatal("pick rebuilt stored child", err)
	}
	present, err := domain.Contains(ctx, entries[0].Value)
	if err != nil || !present {
		t.Fatal("explicit contains", err)
	}
	size, err := domain.Cardinality(ctx)
	if err != nil || size.String() != "2" {
		t.Fatal("explicit cardinality", size, err)
	}
	copyDomain, _ := RuntimeDomainFromValue(normal)
	if equal, err := domain.Equal(ctx, copyDomain); err != nil || !equal {
		t.Fatal("explicit equality", err)
	}
	if subset, err := domain.Subset(ctx, copyDomain); err != nil || !subset {
		t.Fatal("explicit subset", err)
	}
	if got, err := domain.Expression(ctx); err != nil || !reflect.DeepEqual(got, value.Expression()) {
		t.Fatal("explicit expression", err)
	}
	if hash, err := domain.referenceHashCode(); err != nil {
		t.Fatal("explicit hash", hash, err)
	}
	bounds, _ := domain.Bounds(ctx)
	bounds[0].Value = Integer{}
	fresh, _ := domain.Bounds(ctx)
	if fresh[0].Value.String() != "2" {
		t.Fatal("bounds aliases input")
	}
	falseValue, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: false})
	if present, err := domain.Contains(ctx, falseValue); err != nil || present {
		t.Fatal("absent explicit value", err)
	}
	integerSet, _ := NewRuntimeValue(Expr{Kind: "app", Opcode: "Set", Args: []Expr{{Kind: "int", Int: domainFixtureInteger(t, "2")}, {Kind: "int", Int: domainFixtureInteger(t, "1")}}})
	integerDomain, _ := RuntimeDomainFromValue(integerSet)
	interval := NewRuntimeInterval(domainFixtureInteger(t, "1"), domainFixtureInteger(t, "2"))
	if equal, err := interval.Equal(ctx, integerDomain); err != nil || !equal {
		t.Fatal("interval explicit equality", err)
	}
	if equal, err := integerDomain.Equal(ctx, interval); err != nil || !equal {
		t.Fatal("explicit interval equality", err)
	}
	large := NewRuntimeInterval(domainFixtureInteger(t, "1"), domainFixtureInteger(t, "100000000000000000000000000000000000000000000"))
	if equal, err := large.Equal(ctx, integerDomain); err != nil || equal {
		t.Fatal("large unequal domain required enumeration", err)
	}
}

func TestRuntimeDomainCancellationAndValidation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	zero, one := Integer{}, domainFixtureInteger(t, "1")
	interval := NewRuntimeInterval(zero, one)
	empty := NewRuntimeInterval(one, zero)
	natural, _ := NewRuntimeInfiniteSet("Nat")
	integer, _ := NewRuntimeValue(Expr{Kind: "int", Int: one})
	if _, err := RuntimeDomainFromValue(integer); err == nil {
		t.Fatal("scalar domain accepted")
	}
	if _, err := NewRuntimeInfiniteSet("Bool"); err == nil {
		t.Fatal("unknown infinite domain accepted")
	}
	if _, err := interval.Contains(ctx, RuntimeValue{}); err == nil {
		t.Fatal("invalid contains accepted")
	}
	if _, err := interval.Subset(ctx, RuntimeDomain{}); err == nil {
		t.Fatal("invalid subset accepted")
	}
	if _, err := interval.Equal(ctx, RuntimeDomain{}); err == nil {
		t.Fatal("invalid equality accepted")
	}
	if err := interval.ForEach(ctx, nil); err == nil {
		t.Fatal("nil callback accepted")
	}
	if _, err := interval.Sample(ctx, nil); err == nil {
		t.Fatal("nil RNG accepted")
	}
	rng := NewRNG(one)
	if _, err := empty.Sample(ctx, rng); err == nil || rng.State() != one {
		t.Fatal("empty sample consumed RNG", err, rng.State())
	}
	failure := errors.New("callback failure")
	if err := interval.ForEach(ctx, func(RuntimeValue) (bool, error) { return true, failure }); !errors.Is(err, failure) {
		t.Fatal("callback error lost", err)
	}
	count := 0
	if err := interval.ForEach(ctx, func(RuntimeValue) (bool, error) { count++; return false, nil }); err != nil || count != 1 {
		t.Fatal("early stop", err, count)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, domain := range []RuntimeDomain{{}, interval, natural} {
		if _, err := domain.Cardinality(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("cardinality cancellation", err)
		}
		if _, err := domain.Bounds(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("bounds cancellation", err)
		}
		if _, err := domain.Contains(canceled, integer); !errors.Is(err, context.Canceled) {
			t.Fatal("contains cancellation", err)
		}
		if err := domain.ForEach(canceled, func(RuntimeValue) (bool, error) { return true, nil }); !errors.Is(err, context.Canceled) {
			t.Fatal("iteration cancellation", err)
		}
		if _, err := domain.Subset(canceled, interval); !errors.Is(err, context.Canceled) {
			t.Fatal("subset cancellation", err)
		}
		if _, err := domain.Equal(canceled, interval); !errors.Is(err, context.Canceled) {
			t.Fatal("equality cancellation", err)
		}
		if _, err := domain.Pick(canceled, nil); !errors.Is(err, context.Canceled) {
			t.Fatal("pick cancellation", err)
		}
		if _, err := domain.Sample(canceled, rng); !errors.Is(err, context.Canceled) || rng.State() != one {
			t.Fatal("sample cancellation", err)
		}
		if _, err := domain.Materialize(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("normalization cancellation", err)
		}
		if _, err := domain.Expression(canceled); !errors.Is(err, context.Canceled) {
			t.Fatal("display cancellation", err)
		}
	}
	var invalid RuntimeDomain
	if _, err := invalid.Cardinality(ctx); err == nil {
		t.Fatal("invalid domain cardinality")
	}
	if _, err := invalid.referenceHashCode(); err == nil {
		t.Fatal("invalid domain hash")
	}
	large := NewRuntimeInterval(zero, IntegerFromBig(new(big.Int).Lsh(big.NewInt(1), 128)))
	iterctx, itercancel := context.WithCancel(ctx)
	count = 0
	if err := large.ForEach(iterctx, func(RuntimeValue) (bool, error) { count++; itercancel(); return true, nil }); !errors.Is(err, context.Canceled) || count != 1 {
		t.Fatal("large iteration cancellation", err, count)
	}
}
