package quint

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

type compoundDomainSpec struct {
	Kind, Name, First, Last string
	Value                   json.RawMessage
	Args                    json.RawMessage
	Base, Domain, Range     *compoundDomainSpec
}

func compoundSpecIsDomain(spec compoundDomainSpec) bool {
	switch spec.Kind {
	case "Set", "Interval", "InfSet", "CrossProd", "Powerset", "MapSet":
		return true
	}
	return false
}

func compoundFixtureChildren(t *testing.T, spec compoundDomainSpec) []compoundDomainSpec {
	t.Helper()
	var children []compoundDomainSpec
	if err := json.Unmarshal(spec.Args, &children); err != nil {
		t.Fatal("constructor children", spec.Kind, err)
	}
	return children
}

func compoundFixtureValue(t *testing.T, spec compoundDomainSpec) RuntimeValue {
	t.Helper()
	var expression Expr
	switch spec.Kind {
	case "int", "str":
		var text string
		if err := json.Unmarshal(spec.Value, &text); err != nil {
			t.Fatal("scalar spec", err)
		}
		if spec.Kind == "int" {
			expression = Expr{Kind: "int", Int: domainFixtureInteger(t, text)}
		} else {
			expression = Expr{Kind: "str", Str: text}
		}
	case "bool":
		var value bool
		if err := json.Unmarshal(spec.Value, &value); err != nil {
			t.Fatal("Boolean spec", err)
		}
		expression = Expr{Kind: "bool", Bool: value}
	case "Set", "Tup", "List":
		children := compoundFixtureChildren(t, spec)
		values := make([]RuntimeValue, len(children))
		for i, child := range children {
			values[i] = compoundFixtureValue(t, child)
		}
		value, err := newRuntimeComposite(spec.Kind, values)
		if err != nil {
			t.Fatal("collection spec", err)
		}
		return value
	case "Map":
		var pairs [][]compoundDomainSpec
		if err := json.Unmarshal(spec.Args, &pairs); err != nil {
			t.Fatal("map spec", err)
		}
		values := make([]RuntimeValue, len(pairs))
		for i, pair := range pairs {
			if len(pair) != 2 {
				t.Fatal("map pair size", len(pair))
			}
			var err error
			values[i], err = newRuntimeComposite("Tup", []RuntimeValue{compoundFixtureValue(t, pair[0]), compoundFixtureValue(t, pair[1])})
			if err != nil {
				t.Fatal("map pair", err)
			}
		}
		value, err := newRuntimeComposite("Map", values)
		if err != nil {
			t.Fatal("map constructor", err)
		}
		return value
	default:
		t.Fatal("unsupported concrete spec", spec.Kind)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal("scalar constructor", err)
	}
	return value
}

func compoundFixtureDomain(t *testing.T, spec compoundDomainSpec) RuntimeDomain {
	t.Helper()
	var value RuntimeDomain
	var err error
	switch spec.Kind {
	case "Interval":
		return NewRuntimeInterval(domainFixtureInteger(t, spec.First), domainFixtureInteger(t, spec.Last))
	case "InfSet":
		value, err = NewRuntimeInfiniteSet(spec.Name)
	case "Set":
		value, err = RuntimeDomainFromValue(compoundFixtureValue(t, spec))
	case "CrossProd":
		children := compoundFixtureChildren(t, spec)
		factors := make([]RuntimeDomain, len(children))
		for i, child := range children {
			factors[i] = compoundFixtureDomain(t, child)
		}
		value, err = NewRuntimeProduct(factors)
	case "Powerset":
		if spec.Base == nil {
			t.Fatal("missing powerset base")
		}
		value, err = NewRuntimePowerset(compoundFixtureDomain(t, *spec.Base))
	case "MapSet":
		if spec.Domain == nil || spec.Range == nil {
			t.Fatal("missing mapset domain/range")
		}
		value, err = NewRuntimeMapSet(compoundFixtureDomain(t, *spec.Domain), compoundFixtureDomain(t, *spec.Range))
	default:
		t.Fatal("unsupported domain spec", spec.Kind)
	}
	if err != nil {
		t.Fatal("domain constructor", spec.Kind, err)
	}
	return value
}

func compoundFixtureContains(t *testing.T, domain RuntimeDomain, spec compoundDomainSpec) (bool, error) {
	t.Helper()
	if compoundSpecIsDomain(spec) {
		return domain.ContainsDomain(context.Background(), compoundFixtureDomain(t, spec))
	}
	return domain.Contains(context.Background(), compoundFixtureValue(t, spec))
}

func compoundFixtureExpression(t *testing.T, value RuntimeValue, operationError error) (Expr, error) {
	t.Helper()
	if operationError != nil {
		return Expr{}, operationError
	}
	return SerializeRuntimeValue(value, nil)
}

type compoundDomainSnapshot struct {
	Name     string
	Input    compoundDomainSpec
	Expected struct {
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
		Input compoundDomainSpec
		domainFixtureOutcome[bool]
	}
	Picks []struct {
		Positions []string
		Consumed  int
		domainFixtureOutcome[Expr]
	}
	Samples []struct {
		Seed  string
		Draws []struct {
			Positions []Integer
			Picked    bool
			Consumed  int
			State     string
			domainFixtureOutcome[Expr]
		}
	}
}

type compoundDomainTarget struct {
	Name, Operation string
	Input, Other    compoundDomainSpec
	Positions       []string
	Consumed        int
	Value           json.RawMessage
	Diagnostic      *Diagnostic
	Thrown          *RuntimeDomainException
}

func compoundTargetOutcome[T any](t *testing.T, target compoundDomainTarget) domainFixtureOutcome[T] {
	t.Helper()
	want := domainFixtureOutcome[T]{Diagnostic: target.Diagnostic, Thrown: target.Thrown}
	if len(target.Value) > 0 {
		var value T
		if err := json.Unmarshal(target.Value, &value); err != nil {
			t.Fatal("target outcome", err)
		}
		want.Value = &value
	}
	return want
}

func compoundSerializedBounds(bounds []RuntimeBound) []*Integer {
	result := make([]*Integer, len(bounds))
	for i, bound := range bounds {
		if bound.Finite {
			n := bound.Value
			result[i] = &n
		}
	}
	return result
}

func compoundFixturePositions(t *testing.T, values []string) []Integer {
	t.Helper()
	positions := make([]Integer, len(values))
	for i, value := range values {
		positions[i] = domainFixtureInteger(t, value)
	}
	return positions
}

func TestUpstreamCompoundRuntimeDomains(t *testing.T) {
	t.Parallel()
	var fixtures struct {
		Domains []compoundDomainSnapshot
		Cases   []compoundDomainTarget
	}
	data, err := os.ReadFile("testdata/parser/runtime_domain_compound.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range fixtures.Domains {
		t.Run("domain/"+fixture.Name, func(t *testing.T) {
			ctx := context.Background()
			domain := compoundFixtureDomain(t, fixture.Input)
			hash, err := domain.referenceHashCode()
			checkDomainOutcome(t, hash, err, fixture.Expected.HashCode)
			checkDomainOutcome(t, int64(runtimeReferenceSMI(uint32(hash))), err, fixture.Expected.ImmutableHash)
			size, err := domain.Cardinality(ctx)
			checkDomainOutcome(t, size, err, fixture.Expected.Cardinality)
			bounds, err := domain.Bounds(ctx)
			checkDomainOutcome(t, compoundSerializedBounds(bounds), err, fixture.Expected.Bounds)
			expression, err := domain.Expression(ctx)
			checkDomainOutcome(t, expression, err, fixture.Expected.Display)
			iteration := []Expr{}
			err = domain.ForEach(ctx, func(value RuntimeValue) (bool, error) {
				expression, err := SerializeRuntimeValue(value, nil)
				if err != nil {
					return false, err
				}
				iteration = append(iteration, expression)
				return true, nil
			})
			checkDomainOutcome(t, iteration, err, fixture.Expected.Iteration)
			normal, err := domain.Materialize(ctx)
			if fixture.Expected.NormalForm.Thrown != nil {
				checkDomainOutcome(t, struct{}{}, err, domainFixtureOutcome[struct{}]{Thrown: fixture.Expected.NormalForm.Thrown})
			} else if fixture.Expected.NormalForm.Diagnostic != nil {
				checkDomainOutcome(t, struct{}{}, err, domainFixtureOutcome[struct{}]{Diagnostic: fixture.Expected.NormalForm.Diagnostic})
			} else {
				if err != nil || fixture.Expected.NormalForm.Value == nil {
					t.Fatal("normal form", err)
				}
				want := fixture.Expected.NormalForm.Value
				actual, err := SerializeRuntimeValue(normal, nil)
				if err != nil || !reflect.DeepEqual(actual, want.Value) {
					t.Fatal("normal form display", actual, want.Value, err)
				}
				actualHash, err := runtimeReferenceHashCode(normal.expression)
				if err != nil || actualHash != want.HashCode {
					t.Fatal("normal form hash", actualHash, want.HashCode, err)
				}
				entries, err := runtimeEntries(normal)
				if err != nil {
					t.Fatal("normal form entries", err)
				}
				values := make([]Expr, 0, len(entries))
				for _, entry := range entries {
					expression, err := SerializeRuntimeValue(entry.Value, nil)
					if err != nil {
						t.Fatal("normal form child", err)
					}
					values = append(values, expression)
				}
				if !reflect.DeepEqual(values, want.Iteration) {
					t.Fatal("normal form traversal", values, want.Iteration)
				}
			}
			for _, query := range fixture.Contains {
				got, err := compoundFixtureContains(t, domain, query.Input)
				checkDomainOutcome(t, got, err, query.domainFixtureOutcome)
			}
			for _, query := range fixture.Subset {
				got, err := domain.Subset(ctx, compoundFixtureDomain(t, query.Input))
				checkDomainOutcome(t, got, err, query.domainFixtureOutcome)
			}
			for _, query := range fixture.Equals {
				got, err := domain.Equal(ctx, compoundFixtureDomain(t, query.Input))
				checkDomainOutcome(t, got, err, query.domainFixtureOutcome)
			}
			for _, pick := range fixture.Picks {
				value, consumed, err := domain.PickWithPositions(ctx, compoundFixturePositions(t, pick.Positions))
				expression, err := compoundFixtureExpression(t, value, err)
				checkDomainOutcome(t, expression, err, pick.domainFixtureOutcome)
				if consumed != pick.Consumed {
					t.Fatal("positions consumed", consumed, pick.Consumed)
				}
			}
			for _, sample := range fixture.Samples {
				rng := NewRNG(domainFixtureInteger(t, sample.Seed))
				for _, draw := range sample.Draws {
					value, err := domain.Sample(ctx, rng)
					expression, err := compoundFixtureExpression(t, value, err)
					checkDomainOutcome(t, expression, err, draw.domainFixtureOutcome)
					if rng.State().String() != draw.State {
						t.Fatal("sample RNG state", rng.State(), draw.State)
					}
					if draw.Picked {
						at, consumed, err := domain.PickWithPositions(ctx, draw.Positions)
						expression, err := compoundFixtureExpression(t, at, err)
						checkDomainOutcome(t, expression, err, draw.domainFixtureOutcome)
						if consumed != draw.Consumed {
							t.Fatal("sample positions consumed", consumed, draw.Consumed)
						}
					}
				}
			}
		})
	}
	for _, target := range fixtures.Cases {
		t.Run("target/"+target.Name, func(t *testing.T) {
			ctx := context.Background()
			domain := compoundFixtureDomain(t, target.Input)
			switch target.Operation {
			case "cardinality":
				got, err := domain.Cardinality(ctx)
				checkDomainOutcome(t, got, err, compoundTargetOutcome[Integer](t, target))
			case "bounds":
				got, err := domain.Bounds(ctx)
				checkDomainOutcome(t, compoundSerializedBounds(got), err, compoundTargetOutcome[[]*Integer](t, target))
			case "display":
				got, err := domain.Expression(ctx)
				checkDomainOutcome(t, got, err, compoundTargetOutcome[Expr](t, target))
			case "contains":
				got, err := compoundFixtureContains(t, domain, target.Other)
				checkDomainOutcome(t, got, err, compoundTargetOutcome[bool](t, target))
			case "equals":
				got, err := domain.Equal(ctx, compoundFixtureDomain(t, target.Other))
				checkDomainOutcome(t, got, err, compoundTargetOutcome[bool](t, target))
			case "subset":
				got, err := domain.Subset(ctx, compoundFixtureDomain(t, target.Other))
				checkDomainOutcome(t, got, err, compoundTargetOutcome[bool](t, target))
			case "pick":
				got, consumed, err := domain.PickWithPositions(ctx, compoundFixturePositions(t, target.Positions))
				expression, err := compoundFixtureExpression(t, got, err)
				checkDomainOutcome(t, expression, err, compoundTargetOutcome[Expr](t, target))
				if consumed != target.Consumed {
					t.Fatal("target positions consumed", consumed, target.Consumed)
				}
			default:
				t.Fatal("unknown target operation", target.Operation)
			}
		})
	}
}
