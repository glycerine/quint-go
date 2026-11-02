package quint

import (
	"context"
	"errors"
	"testing"
)

func contextAccessValue(t *testing.T, source string) RuntimeValue {
	t.Helper()
	expression, err := ParseExpression(source)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(expression)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextAccessComposite(t *testing.T, op string, args ...RuntimeValue) RuntimeValue {
	t.Helper()
	value, err := NewRuntimeCompositeContext(context.Background(), op, args)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextAccessInterval(t *testing.T, first, last string) RuntimeValue {
	t.Helper()
	start, err := ParseInteger(first)
	if err != nil {
		t.Fatal(err)
	}
	end, err := ParseInteger(last)
	if err != nil {
		t.Fatal(err)
	}
	value, err := RuntimeValueFromDomain(NewRuntimeInterval(start, end))
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextAccessDiagnostic(t *testing.T, err error, code, message string) {
	t.Helper()
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != code || diagnostic.Message != message {
		t.Fatalf("got %v, want %s: %s", err, code, message)
	}
}

func TestContextSequenceAccessRetainsCompactChildren(t *testing.T) {
	ctx := context.Background()
	huge := contextAccessInterval(t, "-1000000000000000000000000000", "1000000000000000000000000000")
	zero, one := contextAccessValue(t, "0"), contextAccessValue(t, "1")
	label := contextAccessValue(t, `"field"`)
	list := contextAccessComposite(t, "List", huge, one)
	tuple := contextAccessComposite(t, "Tup", huge)
	record := contextAccessComposite(t, "Rec", label, huge)
	for _, call := range []struct {
		op   string
		args []RuntimeValue
	}{{"head", []RuntimeValue{list}}, {"nth", []RuntimeValue{list, zero}}, {"item", []RuntimeValue{tuple, one}}, {"field", []RuntimeValue{record, label}}} {
		got, err := EvalSequenceBuiltinContext(ctx, call.op, call.args)
		if err != nil || !sameRuntimeObject(got, huge) {
			t.Fatalf("%s lost retained child: %v", call.op, err)
		}
	}
	for _, call := range []struct {
		op   string
		args []RuntimeValue
		want int
	}{{"tail", []RuntimeValue{contextAccessComposite(t, "List", one, huge)}, 1}, {"append", []RuntimeValue{list, huge}, 3}, {"concat", []RuntimeValue{list, contextAccessComposite(t, "List", huge)}, 3}, {"replaceAt", []RuntimeValue{list, one, huge}, 2}, {"slice", []RuntimeValue{list, zero, one}, 1}} {
		got, err := EvalSequenceBuiltinContext(ctx, call.op, call.args)
		if err != nil {
			t.Fatal(call.op, err)
		}
		children, err := runtimeChildren(got)
		if err != nil || len(children) != call.want || !sameRuntimeObject(children[len(children)-1], huge) {
			t.Fatalf("%s lost updated child: %v", call.op, err)
		}
	}
	updated, err := EvalSequenceBuiltinContext(ctx, "with", []RuntimeValue{record, label, one})
	if err != nil {
		t.Fatal(err)
	}
	got, err := EvalSequenceBuiltinContext(ctx, "field", []RuntimeValue{updated, label})
	if err != nil || !sameRuntimeObject(got, one) {
		t.Fatal("record update", err)
	}
	original, _ := EvalSequenceBuiltinContext(ctx, "field", []RuntimeValue{record, label})
	if !sameRuntimeObject(original, huge) {
		t.Fatal("record update mutated original")
	}
	length, err := EvalSequenceBuiltinContext(ctx, "length", []RuntimeValue{list})
	if err != nil || length.String() != "2" {
		t.Fatal(length.String(), err)
	}
	names, err := EvalSequenceBuiltinContext(ctx, "fieldNames", []RuntimeValue{record})
	if err != nil || names.String() != `Set("field")` {
		t.Fatal(names.String(), err)
	}
	children, _ := runtimeChildren(list)
	children[0] = one
	original, _ = EvalSequenceBuiltinContext(ctx, "head", []RuntimeValue{list})
	if !sameRuntimeObject(original, huge) {
		t.Fatal("returned children alias list storage")
	}
}

func TestContextIndicesAreCompactAndEmptySafe(t *testing.T) {
	ctx := context.Background()
	for _, source := range []string{"List()", "List(1)", "List(1,2,3)"} {
		list := contextAccessValue(t, source)
		got, err := EvalSequenceBuiltinContext(ctx, "indices", []RuntimeValue{list})
		if err != nil || got.domain == nil {
			t.Fatalf("%s indices not compact: %v", source, err)
		}
		materialized, err := got.domain.Materialize(ctx)
		if err != nil {
			t.Fatal(err)
		}
		want, err := EvalSequenceBuiltin("indices", []RuntimeValue{list})
		if err != nil || !materialized.Equal(want) {
			t.Fatalf("%s: %s != %s (%v)", source, materialized.String(), want.String(), err)
		}
	}
}

func TestContextMapKeysNormalizeAndValuesRetainIdentity(t *testing.T) {
	ctx := context.Background()
	key := contextAccessInterval(t, "1", "2")
	huge := contextAccessInterval(t, "0", "999999999999999999999999999")
	mapValue := contextAccessComposite(t, "Map", contextAccessComposite(t, "Tup", key, huge))
	got, err := EvalMapBuiltinContext(ctx, "get", []RuntimeValue{mapValue, key})
	if err != nil || !sameRuntimeObject(got, huge) {
		t.Fatal("lookup changed symbolic child", err)
	}
	keys, err := EvalMapBuiltinContext(ctx, "keys", []RuntimeValue{mapValue})
	if err != nil || keys.containsDomain || keys.String() != "Set(Set(1, 2))" {
		t.Fatal(keys.String(), err)
	}
	before, _ := runtimeMap(mapValue)
	noOp, err := EvalMapBuiltinContext(ctx, "set", []RuntimeValue{mapValue, key, huge})
	after, _ := runtimeMap(noOp)
	if err != nil || before.root != after.root {
		t.Fatal("same-object update rebuilt persistent tree", err)
	}
	replacement := contextAccessInterval(t, "8", "9")
	for _, op := range []string{"set", "put"} {
		updated, err := EvalMapBuiltinContext(ctx, op, []RuntimeValue{mapValue, key, replacement})
		if err != nil {
			t.Fatal(op, err)
		}
		got, err := EvalMapBuiltinContext(ctx, "get", []RuntimeValue{updated, key})
		if err != nil || !sameRuntimeObject(got, replacement) {
			t.Fatal(op, "changed replacement", err)
		}
	}
	got, _ = EvalMapBuiltinContext(ctx, "get", []RuntimeValue{mapValue, key})
	if !sameRuntimeObject(got, huge) {
		t.Fatal("update mutated original map")
	}
	missing := contextAccessValue(t, "3")
	_, err = EvalMapBuiltinContext(ctx, "get", []RuntimeValue{mapValue, missing})
	contextAccessDiagnostic(t, err, "QNT507", "Called 'get' with a non-existing key. Key is 3. Map has keys: Set(1, 2)")
	_, err = EvalMapBuiltinContext(ctx, "set", []RuntimeValue{mapValue, missing, replacement})
	contextAccessDiagnostic(t, err, "QNT507", "Called 'set' with a non-existing key")
	added, err := EvalMapBuiltinContext(ctx, "put", []RuntimeValue{mapValue, missing, replacement})
	if err != nil {
		t.Fatal(err)
	}
	got, err = EvalMapBuiltinContext(ctx, "get", []RuntimeValue{added, missing})
	if err != nil || !sameRuntimeObject(got, replacement) {
		t.Fatal("put failed to add a key", err)
	}
	calls := 0
	callback := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		calls++
		if !sameRuntimeObject(args[0], huge) {
			t.Fatal("setBy changed callback argument")
		}
		return replacement, nil
	})
	updated, err := EvalMapSetByContext(ctx, []RuntimeValue{mapValue, key, callback})
	got, _ = EvalMapBuiltinContext(ctx, "get", []RuntimeValue{updated, key})
	if err != nil || calls != 1 || !sameRuntimeObject(got, replacement) {
		t.Fatal("setBy", err, calls)
	}
	_, err = EvalMapSetByContext(ctx, []RuntimeValue{mapValue, missing, callback})
	contextAccessDiagnostic(t, err, "QNT507", "Called 'setBy' with a non- existing key [object Object]")
	if calls != 1 {
		t.Fatal("missing setBy invoked callback")
	}
	variant := contextAccessComposite(t, "variant", contextAccessValue(t, `"symbolic"`), key)
	_, err = EvalMapBuiltinContext(ctx, "get", []RuntimeValue{mapValue, variant})
	contextAccessDiagnostic(t, err, "QNT501", "Symbolic runtime values retained inside collection keys are not supported")
}

func TestContextListCallbacksPreserveChildrenAndEvaluationOrder(t *testing.T) {
	ctx := context.Background()
	a, b := contextAccessInterval(t, "1", "2"), contextAccessInterval(t, "3", "4")
	list := contextAccessComposite(t, "List", a, b)
	calls := []RuntimeValue{}
	predicate := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		calls = append(calls, args[0])
		return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	})
	selected, err := EvalListHigherBuiltinContext(ctx, "select", []RuntimeValue{list, predicate})
	children, _ := runtimeChildren(selected)
	if err != nil || len(calls) != 2 || len(children) != 2 || !sameRuntimeObject(calls[0], a) || !sameRuntimeObject(calls[1], b) || !sameRuntimeObject(children[0], a) || !sameRuntimeObject(children[1], b) {
		t.Fatal("select changed child objects", err)
	}
	for _, op := range []string{"foldl", "foldr"} {
		calls = nil
		callback := newRuntimeClosure(2, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
			index := 1
			if op == "foldr" {
				index = 0
			}
			calls = append(calls, args[index])
			return args[index], nil
		})
		got, err := EvalListHigherBuiltinContext(ctx, op, []RuntimeValue{list, a, callback})
		first, last := a, b
		if op == "foldr" {
			first, last = b, a
		}
		if err != nil || len(calls) != 2 || !sameRuntimeObject(calls[0], first) || !sameRuntimeObject(calls[1], last) || !sameRuntimeObject(got, last) {
			t.Fatal(op, "callback order or result changed", err)
		}
		empty := contextAccessValue(t, "List()")
		got, err = EvalListHigherBuiltinContext(ctx, op, []RuntimeValue{empty, a, b})
		if err != nil || !sameRuntimeObject(got, a) {
			t.Fatal(op, "empty fold resolved callback", err)
		}
	}
	_, err = EvalListHigherBuiltinContext(ctx, "select", []RuntimeValue{list, a})
	contextAccessDiagnostic(t, err, "QNT501", "Expected a lambda value")
	nonBoolean := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return a, nil })
	_, err = EvalListHigherBuiltinContext(ctx, "select", []RuntimeValue{list, nonBoolean})
	contextAccessDiagnostic(t, err, "QNT501", "Expected a Boolean value")
	failure := Diagnostic{Code: "QNT503", Message: "callback failure"}
	failing := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return RuntimeValue{}, failure })
	_, err = EvalListHigherBuiltinContext(ctx, "select", []RuntimeValue{list, failing})
	contextAccessDiagnostic(t, err, failure.Code, failure.Message)
	for _, op := range []string{"foldl", "foldr"} {
		_, err = EvalListHigherBuiltinContext(ctx, op, []RuntimeValue{list, a, failing})
		contextAccessDiagnostic(t, err, failure.Code, failure.Message)
	}
}

func TestContextSingletonUsesReferenceMaterialization(t *testing.T) {
	ctx := context.Background()
	singleton := contextAccessInterval(t, "7", "7")
	got, err := EvalSetConversionBuiltinContext(ctx, "getOnlyElement", []RuntimeValue{singleton})
	if err != nil || got.String() != "7" {
		t.Fatal(got.String(), err)
	}
	explicit := contextAccessValue(t, "Set(7)")
	got, err = EvalSetConversionBuiltinContext(ctx, "getOnlyElement", []RuntimeValue{explicit})
	entries, _ := runtimeEntries(explicit)
	if err != nil || !sameRuntimeObject(got, entries[0].Value) {
		t.Fatal("explicit singleton lost retained object", err)
	}
	for _, call := range []struct{ first, last, count string }{{"2", "1", "0"}, {"1", "2", "2"}} {
		set := contextAccessInterval(t, call.first, call.last)
		_, err := EvalSetConversionBuiltinContext(ctx, "getOnlyElement", []RuntimeValue{set})
		contextAccessDiagnostic(t, err, "QNT505", "Called 'getOnlyElement' on a set with "+call.count+" elements. Make sure the set has exactly one element.")
	}
	product, err := NewRuntimeProduct(nil)
	if err != nil {
		t.Fatal(err)
	}
	emptyProduct, err := RuntimeValueFromDomain(product)
	if err != nil {
		t.Fatal(err)
	}
	_, err = EvalSetConversionBuiltinContext(ctx, "getOnlyElement", []RuntimeValue{emptyProduct})
	contextAccessDiagnostic(t, err, "QNT505", "Called 'getOnlyElement' on a set with 0 elements. Make sure the set has exactly one element.")
	for _, name := range []string{"Int", "Nat"} {
		domain, err := NewRuntimeInfiniteSet(name)
		if err != nil {
			t.Fatal(err)
		}
		value, err := RuntimeValueFromDomain(domain)
		if err != nil {
			t.Fatal(err)
		}
		_, err = EvalSetConversionBuiltinContext(ctx, "getOnlyElement", []RuntimeValue{value})
		var exception RuntimeDomainException
		if !errors.As(err, &exception) || exception.Name != "Error" || exception.Message != "Infinite set "+name+" is non-enumerable" {
			t.Fatal(name, err)
		}
	}
}

func TestContextAccessCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, call := range []func() (RuntimeValue, error){
		func() (RuntimeValue, error) { return EvalMapBuiltinContext(ctx, "get", nil) },
		func() (RuntimeValue, error) { return EvalMapSetByContext(ctx, nil) },
		func() (RuntimeValue, error) { return EvalSequenceBuiltinContext(ctx, "indices", nil) },
		func() (RuntimeValue, error) { return EvalListHigherBuiltinContext(ctx, "select", nil) },
		func() (RuntimeValue, error) { return EvalSetConversionBuiltinContext(ctx, "getOnlyElement", nil) },
	} {
		if _, err := call(); !errors.Is(err, context.Canceled) {
			t.Fatal("canceled helper accepted work", err)
		}
	}
	ctx, cancel = context.WithCancel(context.Background())
	a := contextAccessInterval(t, "1", "2")
	list := contextAccessComposite(t, "List", a, a)
	calls := 0
	predicate := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
		calls++
		cancel()
		return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	})
	_, err := EvalListHigherBuiltinContext(ctx, "select", []RuntimeValue{list, predicate})
	if !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatal("select did not stop after cancellation", calls, err)
	}
	bounded := &contextAccessCancelContext{Context: context.Background(), remaining: 32}
	huge := contextAccessInterval(t, "0", "1000000000000000000000000000000")
	_, err = EvalSetConversionBuiltinContext(bounded, "getOnlyElement", []RuntimeValue{huge})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("singleton materialization ignored cancellation", err)
	}
}

// A deterministic cancellation context avoids timing-sensitive resource tests.
type contextAccessCancelContext struct {
	context.Context
	remaining int
}

func (ctx *contextAccessCancelContext) Err() error {
	ctx.remaining--
	if ctx.remaining <= 0 {
		return context.Canceled
	}
	return nil
}

func TestContextAccessValidationPreservesConcreteDiagnostics(t *testing.T) {
	ctx := context.Background()
	domain := contextAccessInterval(t, "1", "2")
	list := contextAccessComposite(t, "List", domain)
	record := contextAccessComposite(t, "Rec", contextAccessValue(t, `"field"`), domain)
	mapValue := contextAccessComposite(t, "Map", contextAccessComposite(t, "Tup", contextAccessValue(t, "1"), domain))
	zero, two := contextAccessValue(t, "0"), contextAccessValue(t, "2")
	missing := contextAccessValue(t, `"missing"`)
	for _, test := range []struct {
		name          string
		call          func() (RuntimeValue, error)
		code, message string
	}{
		{"map opcode", func() (RuntimeValue, error) { return EvalMapBuiltinContext(ctx, "unknown", []RuntimeValue{domain}) }, "QNT501", "Runtime does not support the built -in operator 'unknown'"},
		{"map arity", func() (RuntimeValue, error) { return EvalMapBuiltinContext(ctx, "get", []RuntimeValue{domain}) }, "QNT501", "get requires 2 arguments"},
		{"map invalid", func() (RuntimeValue, error) { return EvalMapBuiltinContext(ctx, "get", []RuntimeValue{domain, {}}) }, "QNT501", "Invalid runtime value"},
		{"map shape", func() (RuntimeValue, error) { return EvalMapBuiltinContext(ctx, "get", []RuntimeValue{domain, zero}) }, "QNT501", "get requires a Map value"},
		{"setBy arity", func() (RuntimeValue, error) { return EvalMapSetByContext(ctx, []RuntimeValue{domain}) }, "QNT501", "setBy requires 3 arguments"},
		{"setBy shape", func() (RuntimeValue, error) { return EvalMapSetByContext(ctx, []RuntimeValue{domain, zero, zero}) }, "QNT501", "get requires a Map value"},
		{"sequence opcode", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "unknown", []RuntimeValue{domain})
		}, "QNT501", "Runtime does not support the built -in operator 'unknown'"},
		{"sequence arity", func() (RuntimeValue, error) { return EvalSequenceBuiltinContext(ctx, "nth", []RuntimeValue{domain}) }, "QNT501", "nth requires 2 arguments"},
		{"sequence invalid", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "nth", []RuntimeValue{domain, {}})
		}, "QNT501", "Invalid runtime value"},
		{"sequence shape", func() (RuntimeValue, error) { return EvalSequenceBuiltinContext(ctx, "head", []RuntimeValue{domain}) }, "QNT501", "head requires a List value"},
		{"field type", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "field", []RuntimeValue{record, zero})
		}, "QNT501", "field requires a string field"},
		{"missing field", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "field", []RuntimeValue{record, missing})
		}, "QNT501", "Accessing a missing record field missing"},
		{"missing update field", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "with", []RuntimeValue{record, missing, domain})
		}, "QNT501", "Called 'with' with a non-existent field missing"},
		{"empty head", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "head", []RuntimeValue{contextAccessComposite(t, "List")})
		}, "QNT505", "Called 'head' on an empty list"},
		{"concat shape", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "concat", []RuntimeValue{list, domain})
		}, "QNT501", "concat requires List values"},
		{"index type", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "nth", []RuntimeValue{list, domain})
		}, "QNT501", "nth requires integer indices"},
		{"index bounds", func() (RuntimeValue, error) { return EvalSequenceBuiltinContext(ctx, "nth", []RuntimeValue{list, two}) }, "QNT510", "Out of bounds, nth(2)"},
		{"replace bounds", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "replaceAt", []RuntimeValue{list, two, domain})
		}, "QNT510", "Out of bounds, replaceAt(2)"},
		{"slice type", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "slice", []RuntimeValue{list, zero, domain})
		}, "QNT501", "slice requires integer indices"},
		{"slice bounds", func() (RuntimeValue, error) {
			return EvalSequenceBuiltinContext(ctx, "slice", []RuntimeValue{list, zero, two})
		}, "QNT506", "slice(..., 0, 2) applied to a list of size 1"},
		{"list opcode", func() (RuntimeValue, error) {
			return EvalListHigherBuiltinContext(ctx, "unknown", []RuntimeValue{domain})
		}, "QNT501", "Runtime does not support the built -in operator 'unknown'"},
		{"list arity", func() (RuntimeValue, error) {
			return EvalListHigherBuiltinContext(ctx, "select", []RuntimeValue{domain})
		}, "QNT501", "select requires 2 arguments"},
		{"list invalid", func() (RuntimeValue, error) {
			return EvalListHigherBuiltinContext(ctx, "select", []RuntimeValue{domain, {}})
		}, "QNT501", "Invalid runtime value"},
		{"list shape", func() (RuntimeValue, error) {
			return EvalListHigherBuiltinContext(ctx, "select", []RuntimeValue{domain, zero})
		}, "QNT501", "select requires a List value"},
		{"singleton opcode", func() (RuntimeValue, error) {
			return EvalSetConversionBuiltinContext(ctx, "unknown", []RuntimeValue{domain})
		}, "QNT501", "Runtime does not support the built -in operator 'unknown'"},
		{"singleton arity", func() (RuntimeValue, error) {
			return EvalSetConversionBuiltinContext(ctx, "getOnlyElement", []RuntimeValue{domain, zero})
		}, "QNT501", "getOnlyElement requires 1 argument"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.call()
			contextAccessDiagnostic(t, err, test.code, test.message)
		})
	}
	callback := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
		return RuntimeValue{}, Diagnostic{Code: "QNT503", Message: "callback failure"}
	})
	_, err := EvalMapSetByContext(ctx, []RuntimeValue{mapValue, contextAccessValue(t, "1"), callback})
	contextAccessDiagnostic(t, err, "QNT503", "callback failure")
	concrete := contextAccessValue(t, "Map((1, 0))")
	producesDomain := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return domain, nil })
	updated, err := EvalMapSetByContext(ctx, []RuntimeValue{concrete, contextAccessValue(t, "1"), producesDomain})
	if err != nil {
		t.Fatal("concrete setBy lost symbolic callback result", err)
	}
	got, err := EvalMapBuiltinContext(ctx, "get", []RuntimeValue{updated, contextAccessValue(t, "1")})
	if err != nil || !sameRuntimeObject(got, domain) {
		t.Fatal("setBy changed symbolic callback result", err)
	}
}
