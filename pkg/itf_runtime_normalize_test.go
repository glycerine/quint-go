package quint

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
)

type itfRuntimeNormalizeDescriptor struct {
	Kind, First, Last, Name, Label string
	Value                          json.RawMessage
	Args                           []itfRuntimeNormalizeDescriptor
	Entries                        [][2]json.RawMessage
	Base, Domain, Range            *itfRuntimeNormalizeDescriptor
}

type itfRuntimeNormalizeError struct{ Name, Message string }

type itfRuntimeNormalizeFixture struct {
	Name                                                string
	Input                                               itfRuntimeNormalizeDescriptor
	Direct, Normalized, Wrapped                         *Expr
	DirectError, NormalizationError, SerializationError *itfRuntimeNormalizeError
	ITF                                                 *struct {
		Value json.RawMessage
		Error string
	}
	Counter struct{ Before, AfterNormalization, AfterSerialization, AfterITF uint32 }
}

func itfRuntimeNormalizeInteger(t *testing.T, text string) Integer {
	t.Helper()
	integer, err := ParseInteger(text)
	if err != nil {
		t.Fatal(err)
	}
	return integer
}

func itfRuntimeNormalizeBuild(t *testing.T, descriptor itfRuntimeNormalizeDescriptor) RuntimeValue {
	t.Helper()
	ctx := context.Background()
	var value RuntimeValue
	var err error
	composite := func(op string, children []RuntimeValue) RuntimeValue {
		t.Helper()
		result, err := NewRuntimeCompositeContext(ctx, op, children)
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	decode := func(raw json.RawMessage) itfRuntimeNormalizeDescriptor {
		t.Helper()
		var child itfRuntimeNormalizeDescriptor
		if err := json.Unmarshal(raw, &child); err != nil {
			t.Fatal(err)
		}
		return child
	}
	switch descriptor.Kind {
	case "bool", "int", "str":
		expression := Expr{Kind: descriptor.Kind}
		switch descriptor.Kind {
		case "bool":
			if err := json.Unmarshal(descriptor.Value, &expression.Bool); err != nil {
				t.Fatal(err)
			}
		case "str", "int":
			var text string
			if err := json.Unmarshal(descriptor.Value, &text); err != nil {
				t.Fatal(err)
			}
			if descriptor.Kind == "str" {
				expression.Str = text
			} else {
				expression.Int = itfRuntimeNormalizeInteger(t, text)
			}
		}
		value, err = NewRuntimeValue(expression)
	case "interval":
		value, err = RuntimeValueFromDomain(NewRuntimeInterval(itfRuntimeNormalizeInteger(t, descriptor.First), itfRuntimeNormalizeInteger(t, descriptor.Last)))
	case "infinite":
		var domain RuntimeDomain
		domain, err = NewRuntimeInfiniteSet(descriptor.Name)
		if err == nil {
			value, err = RuntimeValueFromDomain(domain)
		}
	case "set", "list", "tuple", "product":
		children := make([]RuntimeValue, len(descriptor.Args))
		for i, child := range descriptor.Args {
			children[i] = itfRuntimeNormalizeBuild(t, child)
		}
		switch descriptor.Kind {
		case "product":
			value, err = EvalSymbolicBuiltin(ctx, "tuples", children)
		case "set":
			value = composite("Set", children)
		case "list":
			value = composite("List", children)
		case "tuple":
			value = composite("Tup", children)
		}
	case "record":
		children := make([]RuntimeValue, 0, 2*len(descriptor.Entries))
		for _, pair := range descriptor.Entries {
			var key string
			if err := json.Unmarshal(pair[0], &key); err != nil {
				t.Fatal(err)
			}
			name, err := NewRuntimeValue(Expr{Kind: "str", Str: key})
			if err != nil {
				t.Fatal(err)
			}
			children = append(children, name, itfRuntimeNormalizeBuild(t, decode(pair[1])))
		}
		value = composite("Rec", children)
	case "map":
		pairs := make([]RuntimeValue, len(descriptor.Entries))
		for i, pair := range descriptor.Entries {
			pairs[i] = composite("Tup", []RuntimeValue{itfRuntimeNormalizeBuild(t, decode(pair[0])), itfRuntimeNormalizeBuild(t, decode(pair[1]))})
		}
		value = composite("Map", pairs)
	case "variant":
		label, err := NewRuntimeValue(Expr{Kind: "str", Str: descriptor.Label})
		if err != nil {
			t.Fatal(err)
		}
		value = composite("variant", []RuntimeValue{label, itfRuntimeNormalizeBuild(t, decode(descriptor.Value))})
	case "powerset":
		value, err = EvalSymbolicBuiltin(ctx, "powerset", []RuntimeValue{itfRuntimeNormalizeBuild(t, *descriptor.Base)})
	case "mapset":
		value, err = EvalSymbolicBuiltin(ctx, "setOfMaps", []RuntimeValue{itfRuntimeNormalizeBuild(t, *descriptor.Domain), itfRuntimeNormalizeBuild(t, *descriptor.Range)})
	default:
		t.Fatalf("unknown runtime normalization descriptor %q", descriptor.Kind)
	}
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func itfRuntimeNormalizeCheckError(t *testing.T, err error, expected *itfRuntimeNormalizeError) {
	t.Helper()
	if expected == nil {
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	var thrown RuntimeDomainException
	if !errors.As(err, &thrown) || thrown.Name != expected.Name || thrown.Message != expected.Message {
		t.Fatalf("error %v, want %#v", err, expected)
	}
}

func TestUpstreamITFRuntimeNormalization(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("testdata/parser/itf_runtime_normalize.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []itfRuntimeNormalizeFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures) == 0 {
		t.Fatal("empty source normalization oracle")
	}
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			directValue := itfRuntimeNormalizeBuild(t, fixture.Input)
			direct, err := SerializeRuntimeValueContext(ctx, directValue, nil)
			itfRuntimeNormalizeCheckError(t, err, fixture.DirectError)
			if err == nil && (fixture.Direct == nil || !reflect.DeepEqual(direct, *fixture.Direct)) {
				t.Fatalf("direct %s, want %v", ExpressionString(direct), fixture.Direct)
			}
			value := itfRuntimeNormalizeBuild(t, fixture.Input)
			before := value.Expression()
			objects := &RuntimeObjectHashState{}
			if objects.Counter() != fixture.Counter.Before {
				t.Fatal("construction UID", objects.Counter(), fixture.Counter.Before)
			}
			normalized, err := NormalizeRuntimeValueContext(ctx, value)
			itfRuntimeNormalizeCheckError(t, err, fixture.NormalizationError)
			if objects.Counter() != fixture.Counter.AfterNormalization {
				t.Fatal("normalization UID", objects.Counter(), fixture.Counter.AfterNormalization)
			}
			if !reflect.DeepEqual(before, value.Expression()) {
				t.Fatal("normalization changed retained input")
			}
			if err != nil {
				return
			}
			expression, err := SerializeRuntimeValueContext(ctx, normalized, objects)
			itfRuntimeNormalizeCheckError(t, err, fixture.SerializationError)
			if objects.Counter() != fixture.Counter.AfterSerialization {
				t.Fatal("serialization UID", objects.Counter(), fixture.Counter.AfterSerialization)
			}
			if err != nil {
				return
			}
			if fixture.Normalized == nil || !reflect.DeepEqual(expression, *fixture.Normalized) {
				t.Fatalf("normalized %s, want %v", ExpressionString(expression), fixture.Normalized)
			}
			wrapped := Expr{Kind: "app", Opcode: "Rec", Args: []Expr{{Kind: "str", Str: "x"}, expression}}
			if fixture.Wrapped == nil || !reflect.DeepEqual(wrapped, *fixture.Wrapped) {
				t.Fatal("source record wrapper differs")
			}
			encoded, err := EncodeITFTrace([]string{"x"}, []Expr{wrapped}, false)
			if fixture.ITF == nil {
				t.Fatal("missing source ITF outcome")
			}
			if fixture.ITF.Error != "" {
				if err == nil || err.Error() != fixture.ITF.Error {
					t.Fatal("ITF error", err, fixture.ITF.Error)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				var actual, expected any
				if err := json.Unmarshal(encoded, &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(fixture.ITF.Value, &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(actual, expected) {
					t.Fatalf("ITF %s, want %s", encoded, fixture.ITF.Value)
				}
			}
			if objects.Counter() != fixture.Counter.AfterITF {
				t.Fatal("ITF UID", objects.Counter(), fixture.Counter.AfterITF)
			}
		})
	}
}

type itfRuntimeNormalizeCancelContext struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (ctx *itfRuntimeNormalizeCancelContext) Err() error {
	ctx.remaining--
	if ctx.remaining == 0 {
		ctx.cancel()
	}
	return ctx.Context.Err()
}

func TestNormalizeRuntimeValueOwnershipAndCancellation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	interval, err := RuntimeValueFromDomain(NewRuntimeInterval(itfRuntimeNormalizeInteger(t, "9"), itfRuntimeNormalizeInteger(t, "12")))
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := NormalizeRuntimeValueContext(ctx, interval)
	if err != nil || normalized.containsDomain || !interval.containsDomain {
		t.Fatal("compact input ownership", err)
	}
	view := normalized.Expression()
	view.Args[0].Int = Integer{}
	if normalized.Expression().Args[0].Int.String() == "0" {
		t.Fatal("transport view mutated normalized value")
	}
	wrapped, err := NormalizeRuntimeValueContext(ctx, normalized)
	if err != nil || wrapped.payload.entries.root != normalized.payload.entries.root || runtimeReferenceMemoOf(wrapped) != runtimeReferenceMemoOf(normalized) {
		t.Fatal("set normalization must share retained tree and memo", err)
	}
	label, _ := NewRuntimeValue(Expr{Kind: "str", Str: "D"})
	variant, err := NewRuntimeCompositeContext(ctx, "variant", []RuntimeValue{label, interval})
	if err != nil {
		t.Fatal(err)
	}
	opaque, err := NormalizeRuntimeValueContext(ctx, variant)
	if err != nil || !sameRuntimeObject(opaque, variant) || opaque.payload != variant.payload {
		t.Fatal("variant payload should remain opaque", err)
	}
	if _, err := NormalizeRuntimeValueContext(ctx, RuntimeValue{}); err == nil {
		t.Fatal("accepted invalid runtime value")
	} else {
		var diagnostic Diagnostic
		if !errors.As(err, &diagnostic) || diagnostic.Code != "QNT501" || diagnostic.Message != "Invalid runtime value" {
			t.Fatal("invalid value error changed", err)
		}
	}
	called := false
	closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
		called = true
		return normalized, nil
	})
	retained, err := NormalizeRuntimeValueContext(ctx, closure)
	if err != nil || retained.closure != closure.closure {
		t.Fatal("normalization changed executable closure", err)
	}
	if result, err := retained.Apply(ctx, nil); err != nil || !called || !result.Equal(normalized) {
		t.Fatal("normalized closure lost its execution owner", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := NormalizeRuntimeValueContext(canceled, interval); !errors.Is(err, context.Canceled) {
		t.Fatal("entry cancellation", err)
	}
	huge, err := RuntimeValueFromDomain(NewRuntimeInterval(Integer{}, itfRuntimeNormalizeInteger(t, "1000000000000000000000000000000000000")))
	if err != nil {
		t.Fatal(err)
	}
	active, stop := context.WithCancel(ctx)
	defer stop()
	bounded := &itfRuntimeNormalizeCancelContext{Context: active, cancel: stop, remaining: 20}
	if result, err := NormalizeRuntimeValueContext(bounded, huge); !errors.Is(err, context.Canceled) || result.Valid() {
		t.Fatal("cancellation during enumeration", err, result)
	}
}
