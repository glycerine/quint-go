package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
)

type sourceEqualitySpec struct {
	Kind                         string
	Value, Args                  json.RawMessage
	First, Last, Name, Label, ID string
	Arity                        int
	Base, Domain, Range          *sourceEqualitySpec
}
type sourceEqualityOutcome struct {
	Value  *bool
	Thrown *RuntimeDomainException
}

func sourceEqualityValue(t *testing.T, spec sourceEqualitySpec, retained map[string]RuntimeValue) RuntimeValue {
	t.Helper()
	ctx := context.Background()
	var value RuntimeValue
	var err error
	scalar := func(kind string, data json.RawMessage) RuntimeValue {
		var expression Expr
		expression.Kind = kind
		switch kind {
		case "bool":
			err = json.Unmarshal(data, &expression.Bool)
		case "str":
			err = json.Unmarshal(data, &expression.Str)
		case "int":
			var n string
			err = json.Unmarshal(data, &n)
			if err == nil {
				expression.Int, err = ParseInteger(n)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		v, e := NewRuntimeValue(expression)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	children := func() []RuntimeValue {
		var specs []sourceEqualitySpec
		if e := json.Unmarshal(spec.Args, &specs); e != nil {
			t.Fatal(e)
		}
		values := make([]RuntimeValue, len(specs))
		for i, child := range specs {
			values[i] = sourceEqualityValue(t, child, retained)
		}
		return values
	}
	domain := func(s *sourceEqualitySpec) RuntimeDomain {
		if s == nil {
			t.Fatal("missing domain spec")
		}
		v := sourceEqualityValue(t, *s, retained)
		d, e := RuntimeDomainFromValue(v)
		if e != nil {
			t.Fatal(e)
		}
		return d
	}
	switch spec.Kind {
	case "Ref":
		var ok bool
		value, ok = retained[spec.Name]
		if !ok {
			t.Fatalf("missing retained value %s", spec.Name)
		}
	case "Bool":
		value = scalar("bool", spec.Value)
	case "Int":
		value = scalar("int", spec.Value)
	case "Str":
		value = scalar("str", spec.Value)
	case "List", "Tup", "Set":
		value, err = NewRuntimeCompositeContext(ctx, spec.Kind, children())
	case "Rec":
		var pairs [][2]json.RawMessage
		if e := json.Unmarshal(spec.Args, &pairs); e != nil {
			t.Fatal(e)
		}
		args := []RuntimeValue{}
		for _, pair := range pairs {
			var child sourceEqualitySpec
			if e := json.Unmarshal(pair[1], &child); e != nil {
				t.Fatal(e)
			}
			args = append(args, scalar("str", pair[0]), sourceEqualityValue(t, child, retained))
		}
		value, err = NewRuntimeCompositeContext(ctx, "Rec", args)
	case "Map":
		var pairs [][2]sourceEqualitySpec
		if e := json.Unmarshal(spec.Args, &pairs); e != nil {
			t.Fatal(e)
		}
		args := []RuntimeValue{}
		for _, pair := range pairs {
			a, b := sourceEqualityValue(t, pair[0], retained), sourceEqualityValue(t, pair[1], retained)
			p, e := NewRuntimeCompositeContext(ctx, "Tup", []RuntimeValue{a, b})
			if e != nil {
				t.Fatal(e)
			}
			args = append(args, p)
		}
		value, err = NewRuntimeCompositeContext(ctx, "Map", args)
	case "Interval":
		a, e := ParseInteger(spec.First)
		if e != nil {
			t.Fatal(e)
		}
		b, e := ParseInteger(spec.Last)
		if e != nil {
			t.Fatal(e)
		}
		value, err = RuntimeValueFromDomain(NewRuntimeInterval(a, b))
	case "InfSet":
		d, e := NewRuntimeInfiniteSet(spec.Name)
		if e != nil {
			t.Fatal(e)
		}
		value, err = RuntimeValueFromDomain(d)
	case "CrossProd":
		values := children()
		domains := make([]RuntimeDomain, len(values))
		for i, v := range values {
			domains[i], err = RuntimeDomainFromValue(v)
			if err != nil {
				t.Fatal(err)
			}
		}
		d, e := NewRuntimeProduct(domains)
		if e != nil {
			t.Fatal(e)
		}
		value, err = RuntimeValueFromDomain(d)
	case "Powerset":
		d, e := NewRuntimePowerset(domain(spec.Base))
		if e != nil {
			t.Fatal(e)
		}
		value, err = RuntimeValueFromDomain(d)
	case "MapSet":
		d, e := NewRuntimeMapSet(domain(spec.Domain), domain(spec.Range))
		if e != nil {
			t.Fatal(e)
		}
		value, err = RuntimeValueFromDomain(d)
	case "Variant":
		var child sourceEqualitySpec
		if e := json.Unmarshal(spec.Value, &child); e != nil {
			t.Fatal(e)
		}
		label, e := NewRuntimeValue(Expr{Kind: "str", Str: spec.Label})
		if e != nil {
			t.Fatal(e)
		}
		value, err = NewRuntimeCompositeContext(ctx, "variant", []RuntimeValue{label, sourceEqualityValue(t, child, retained)})
	case "Lambda":
		value = newRuntimeClosure(spec.Arity, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
			return NewRuntimeValue(Expr{Kind: "int", Int: IntegerFromBig(domainFixtureInteger(t, "7").Big())})
		})
	default:
		t.Fatalf("unsupported source constructor %s", spec.Kind)
	}
	if err != nil {
		t.Fatalf("constructor %s: %v", spec.Kind, err)
	}
	if spec.ID != "" {
		retained[spec.ID] = value
	}
	return value
}
func sourceEqualityCheck(t *testing.T, got bool, err error, want sourceEqualityOutcome) {
	t.Helper()
	if want.Thrown != nil {
		var exception RuntimeDomainException
		if !errors.As(err, &exception) || exception != *want.Thrown {
			t.Fatalf("got %v/%v, want%+v", got, err, want.Thrown)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if want.Value == nil || got != *want.Value {
		t.Fatalf("got%v, want%+v", got, want.Value)
	}
}
func TestUpstreamSourceConstructorEquality(t *testing.T) {
	t.Parallel()
	var fixture struct {
		Representatives []struct {
			Name  string
			Input sourceEqualitySpec
		}
		Matrix []struct {
			Left, Right string
			Equals      sourceEqualityOutcome
		}
		Comparisons []struct {
			Name                                                 string
			Left, Right                                          sourceEqualitySpec
			SameObject                                           bool
			Equals, ReverseEquals, ImmutableIs, NormalFormEquals sourceEqualityOutcome
		}
		Sources []symbolicExpressionCase
	}
	data, err := os.ReadFile("testdata/parser/runtime_source_equality.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	specs := map[string]sourceEqualitySpec{}
	for _, representative := range fixture.Representatives {
		specs[representative.Name] = representative.Input
	}
	for _, cell := range fixture.Matrix {
		t.Run(cell.Left+"/"+cell.Right, func(t *testing.T) {
			a, b := sourceEqualityValue(t, specs[cell.Left], map[string]RuntimeValue{}), sourceEqualityValue(t, specs[cell.Right], map[string]RuntimeValue{})
			got, err := runtimeSourceEqualContext(context.Background(), a, b)
			sourceEqualityCheck(t, got, err, cell.Equals)
		})
	}
	for _, test := range fixture.Comparisons {
		t.Run(test.Name, func(t *testing.T) {
			retained := map[string]RuntimeValue{}
			a, b := sourceEqualityValue(t, test.Left, retained), sourceEqualityValue(t, test.Right, retained)
			if sameRuntimeObject(a, b) != test.SameObject {
				t.Fatal("retained identity mismatch")
			}
			ctx := context.Background()
			got, err := runtimeSourceEqualContext(ctx, a, b)
			sourceEqualityCheck(t, got, err, test.Equals)
			got, err = runtimeSourceEqualContext(ctx, b, a)
			sourceEqualityCheck(t, got, err, test.ReverseEquals)
			got, err = runtimeSourceCollectionEqualContext(ctx, a, b)
			sourceEqualityCheck(t, got, err, test.ImmutableIs)
			an, err := runtimeNormalFormContext(ctx, a)
			if err != nil {
				sourceEqualityCheck(t, false, err, test.NormalFormEquals)
				return
			}
			bn, err := runtimeNormalFormContext(ctx, b)
			if err != nil {
				sourceEqualityCheck(t, false, err, test.NormalFormEquals)
				return
			}
			got, err = runtimeSourceEqualContext(ctx, an, bn)
			sourceEqualityCheck(t, got, err, test.NormalFormEquals)
		})
	}
	for _, test := range fixture.Sources {
		t.Run(test.Name, func(t *testing.T) { assertSymbolicExpression(t, test) })
	}
}

func TestSourceEqualityKeepsPublicBoundaryAndCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pairs := [][2]RuntimeValue{
		{contextAccessValue(t, "List(1,1)"), contextAccessValue(t, "Set(1)")},
		{contextAccessValue(t, "Map((1,2))"), contextAccessValue(t, "Set()")},
		{contextAccessValue(t, "{key:1}"), contextAccessValue(t, "Map()")},
		{contextAccessValue(t, "List(Map((1,2)))"), contextAccessValue(t, "List(Set())")},
	}
	for _, pair := range pairs {
		oldLeft, oldRight := pair[0].Hash(), pair[1].Hash()
		private, err := runtimeSourceEqualContext(ctx, pair[0], pair[1])
		if err != nil || !private {
			t.Fatalf("source fallback %v/%v", private, err)
		}
		public, err := pair[0].EqualContext(ctx, pair[1])
		if err != nil || public {
			t.Fatalf("public boundary changed %v/%v", public, err)
		}
		if pair[0].Hash() != oldLeft || pair[1].Hash() != oldRight {
			t.Fatal("source fallback changed public hashes")
		}
		a, err := pair[0].HashContext(ctx)
		if err != nil || a != oldLeft {
			t.Fatal("checked concrete hash changed", err)
		}
		b, err := pair[1].HashContext(ctx)
		if err != nil || b != oldRight {
			t.Fatal("checked concrete hash changed", err)
		}
	}
	huge := contextAccessInterval(t, "0", "1000000000000000000000000000000")
	empty := contextAccessValue(t, "Set()")
	limited := &contextIdentityCancelContext{Context: ctx}
	if _, err := runtimeSourceEqualContext(limited, huge, empty); !errors.Is(err, context.Canceled) {
		t.Fatalf("source equality skipped toSet traversal: %v", err)
	}
	limited = &contextIdentityCancelContext{Context: ctx}
	equal, err := huge.EqualContext(limited, empty)
	if err != nil || equal {
		t.Fatalf("public optimized interval boundary %v/%v", equal, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := runtimeSourceEqualContext(canceled, pairs[0][0], pairs[0][1]); !errors.Is(err, context.Canceled) {
		t.Fatalf("entry cancellation %v", err)
	}
	if _, err := runtimeSourceToSetContext(canceled, pairs[0][0]); !errors.Is(err, context.Canceled) {
		t.Fatalf("conversion cancellation %v", err)
	}
	if _, err := runtimeSourceEqualContext(ctx, RuntimeValue{}, empty); err == nil {
		t.Fatal("invalid source argument accepted")
	}
	if _, err := runtimeSourceToSetContext(ctx, contextAccessValue(t, "1")); err == nil {
		t.Fatal("nonsetlike source conversion accepted")
	}
}
