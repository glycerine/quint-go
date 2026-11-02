package quint

import (
	"context"
	"errors"
	"math/big"
	"testing"
)

func contextSetsInteger(t *testing.T, text string) RuntimeValue {
	t.Helper()
	number, err := ParseInteger(text)
	if err != nil {
		t.Fatal(err)
	}
	value, err := NewRuntimeValue(Expr{Kind: "int", Int: number})
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextSetsDomain(t *testing.T, domain RuntimeDomain) RuntimeValue {
	t.Helper()
	value, err := RuntimeValueFromDomain(domain)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextSetsInterval(t *testing.T, first, last string) RuntimeValue {
	t.Helper()
	left, right := contextSetsInteger(t, first), contextSetsInteger(t, last)
	return contextSetsDomain(t, NewRuntimeInterval(left.expression.Int, right.expression.Int))
}

func contextSetsComposite(t *testing.T, op string, values ...RuntimeValue) RuntimeValue {
	t.Helper()
	value, err := NewRuntimeCompositeContext(context.Background(), op, values)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func contextSetsCheckEqual(t *testing.T, got RuntimeValue, err error, want RuntimeValue) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	equal, err := got.EqualContext(context.Background(), want)
	if err != nil || !equal {
		t.Fatal("value", got.String(), want.String(), err)
	}
}

func contextSetsCheckDiagnostic(t *testing.T, err error, code, message string) {
	t.Helper()
	var diagnostic Diagnostic
	if !errors.As(err, &diagnostic) || diagnostic.Code != code || diagnostic.Message != message {
		t.Fatal("diagnostic", err, code, message)
	}
}

func TestContextSetPredicatesAndMaterialization(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	zero, one, two, three, four := contextSetsInteger(t, "0"), contextSetsInteger(t, "1"), contextSetsInteger(t, "2"), contextSetsInteger(t, "3"), contextSetsInteger(t, "4")
	huge := contextSetsInterval(t, "1", "100000000000000000000000000000000000000000000000000")
	got, err := EvalSetBuiltinContext(ctx, "size", []RuntimeValue{huge})
	contextSetsCheckEqual(t, got, err, contextSetsInteger(t, "100000000000000000000000000000000000000000000000000"))
	for _, call := range []struct {
		op   string
		args []RuntimeValue
		want bool
	}{{"contains", []RuntimeValue{huge, one}, true}, {"contains", []RuntimeValue{huge, zero}, false}, {"in", []RuntimeValue{one, huge}, true}} {
		got, err := EvalSetBuiltinContext(ctx, call.op, call.args)
		if err != nil || got.expression.Kind != "bool" || got.expression.Bool != call.want {
			t.Fatal(call.op, got.String(), err)
		}
	}
	integerDomain, _ := NewRuntimeInfiniteSet("Int")
	naturalDomain, _ := NewRuntimeInfiniteSet("Nat")
	integers, naturals := contextSetsDomain(t, integerDomain), contextSetsDomain(t, naturalDomain)
	for _, call := range []struct {
		op   string
		args []RuntimeValue
		want bool
	}{{"isFinite", []RuntimeValue{integers}, true}, {"subseteq", []RuntimeValue{naturals, integers}, true}, {"subseteq", []RuntimeValue{integers, naturals}, false}, {"contains", []RuntimeValue{naturals, contextSetsInteger(t, "-1")}, false}, {"subseteq", []RuntimeValue{contextSetsInterval(t, "9", "2"), contextSetsInterval(t, "2", "3")}, false}} {
		got, err := EvalSetBuiltinContext(ctx, call.op, call.args)
		if err != nil || got.expression.Kind != "bool" || got.expression.Bool != call.want {
			t.Fatal(call.op, got.String(), err)
		}
	}
	_, err = EvalSetBuiltinContext(ctx, "size", []RuntimeValue{integers})
	contextSetsCheckDiagnostic(t, err, "QNT514", "Cardinality of Int is infinite")
	interval := contextSetsInterval(t, "1", "3")
	right := contextSetsComposite(t, "Set", three, four)
	for _, call := range []struct {
		op   string
		want RuntimeValue
	}{{"union", contextSetsComposite(t, "Set", one, two, three, four)}, {"intersect", contextSetsComposite(t, "Set", three)}, {"exclude", contextSetsComposite(t, "Set", one, two)}} {
		got, err := EvalSetBuiltinContext(ctx, call.op, []RuntimeValue{interval, right})
		contextSetsCheckEqual(t, got, err, call.want)
	}
	base, _ := RuntimeDomainFromValue(contextSetsComposite(t, "Set", two, one))
	powerset, err := NewRuntimePowerset(base)
	if err != nil {
		t.Fatal(err)
	}
	got, err = EvalSetBuiltinContext(ctx, "flatten", []RuntimeValue{contextSetsDomain(t, powerset)})
	contextSetsCheckEqual(t, got, err, contextSetsComposite(t, "Set", one, two))
	if huge.domain == nil || huge.expression.Opcode != "to" || !huge.containsDomain {
		t.Fatal("predicate materialized compact input")
	}
	_, err = EvalSetBuiltinContext(ctx, "union", []RuntimeValue{integers, right})
	var exception RuntimeDomainException
	if !errors.As(err, &exception) || exception.Name != "Error" || exception.Message != "Infinite set Int is non-enumerable" {
		t.Fatal("non-enumerable union", err)
	}
}

func TestContextSetHigherCallbacksReturnCompactValues(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	one, two, three := contextSetsInteger(t, "1"), contextSetsInteger(t, "2"), contextSetsInteger(t, "3")
	concrete := contextSetsComposite(t, "Set", one, two)
	calls := 0
	mapper := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		calls++
		first := args[0].expression.Int
		last := IntegerFromBig(new(big.Int).Add(first.Big(), big.NewInt(1)))
		return RuntimeValueFromDomain(NewRuntimeInterval(first, last))
	})
	got, err := EvalSetHigherBuiltinContext(ctx, "map", []RuntimeValue{concrete, mapper})
	contextSetsCheckEqual(t, got, err, contextSetsComposite(t, "Set", contextSetsComposite(t, "Set", one, two), contextSetsComposite(t, "Set", two, three)))
	if calls != 2 || got.containsDomain {
		t.Fatal("mapped domains were not normalized", calls, got.containsDomain)
	}
	huge := contextSetsInterval(t, "1", "100000000000000000000000000000000000000000000000000")
	retained := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return huge, nil })
	got, err = EvalSetHigherBuiltinContext(ctx, "mapBy", []RuntimeValue{contextSetsInterval(t, "1", "3"), retained})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := runtimeEntries(got)
	if err != nil || len(entries) != 3 {
		t.Fatal("mapBy entries", len(entries), err)
	}
	for _, entry := range entries {
		if !sameRuntimeObject(entry.Value, huge) {
			t.Fatal("mapBy rebuilt compact callback value")
		}
	}
	count := 0
	reducer := newRuntimeClosure(2, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		count++
		if !sameRuntimeObject(args[0], huge) || args[1].expression.Kind != "int" {
			t.Fatal("fold lost accumulator or reversed arguments")
		}
		return args[0], nil
	})
	got, err = EvalSetHigherBuiltinContext(ctx, "fold", []RuntimeValue{contextSetsInterval(t, "1", "2"), huge, reducer})
	if err != nil || count != 2 || !sameRuntimeObject(got, huge) {
		t.Fatal("fold rebuilt compact accumulator", err, count)
	}
	got, err = EvalSetHigherBuiltinContext(ctx, "fold", []RuntimeValue{contextSetsInterval(t, "4", "2"), huge, one})
	if err != nil || !sameRuntimeObject(got, huge) {
		t.Fatal("empty fold resolved callback or rebuilt initial value", err)
	}
	predicate := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		return NewRuntimeValue(Expr{Kind: "bool", Bool: args[0].expression.Int.Big().Bit(0) == 1})
	})
	got, err = EvalSetHigherBuiltinContext(ctx, "filter", []RuntimeValue{contextSetsInterval(t, "1", "3"), predicate})
	contextSetsCheckEqual(t, got, err, contextSetsComposite(t, "Set", one, three))
	failure := Diagnostic{Code: "QNT503", Message: "later callback"}
	calls = 0
	eager := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
		calls++
		if calls == 2 {
			return RuntimeValue{}, failure
		}
		return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	})
	_, err = EvalSetHigherBuiltinContext(ctx, "exists", []RuntimeValue{contextSetsInterval(t, "1", "3"), eager})
	contextSetsCheckDiagnostic(t, err, failure.Code, failure.Message)
	if calls != 2 {
		t.Fatal("quantifier short-circuited callback evaluation", calls)
	}
}

func TestContextSetRawMixedRepresentationsCompareContextually(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	one, two := contextSetsInteger(t, "1"), contextSetsInteger(t, "2")
	input := contextSetsComposite(t, "Set", one, two)
	interval := contextSetsInterval(t, "1", "2")
	explicit := contextSetsComposite(t, "Set", one, two)
	mapper := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		if args[0].expression.Int.String() == "1" {
			return interval, nil
		}
		return explicit, nil
	})
	mapped, err := EvalSetHigherBuiltinContext(ctx, "map", []RuntimeValue{input, mapper})
	if err != nil {
		t.Fatal(err)
	}
	mappedEntries, err := runtimeEntries(mapped)
	if err != nil || len(mappedEntries) != 1 || !mappedEntries[0].Value.Equal(explicit) {
		t.Fatalf("mixed representations must normalize and deduplicate: %v", err)
	}
	// mapBy normalizes only keys; different compact value representations are
	// retained freely and do not use the raw-result Set comparison boundary.
	got, err := EvalSetHigherBuiltinContext(ctx, "mapBy", []RuntimeValue{input, mapper})
	if err != nil {
		t.Fatal(err)
	}
	entries, _ := runtimeEntries(got)
	if len(entries) != 2 || !sameRuntimeObject(entries[0].Value, interval) || !sameRuntimeObject(entries[1].Value, explicit) {
		t.Fatal("mapBy rejected or rebuilt mixed retained values")
	}
}

func contextSetsCheckEntryOrder(t *testing.T, got, want RuntimeValue) {
	t.Helper()
	left, err := runtimeEntries(got)
	if err != nil {
		t.Fatal(err)
	}
	right, err := runtimeEntries(want)
	if err != nil || len(left) != len(right) {
		t.Fatal("entry count", len(left), len(right), err)
	}
	for i, entry := range left {
		if !entry.Key.Equal(right[i].Key) || !entry.Value.Equal(right[i].Value) {
			t.Fatal("entry traversal", i, entry.Key.String(), right[i].Key.String(), entry.Value.String(), right[i].Value.String())
		}
	}
}

func TestContextSetConversionsPreserveObjectHashHistory(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	keys := make([]RuntimeValue, 9)
	for i := range keys {
		value := IntegerFromBig(new(big.Int).Mul(big.NewInt(int64(i)), new(big.Int).Lsh(big.NewInt(1), 32)))
		keys[i], _ = NewRuntimeValue(Expr{Kind: "int", Int: value})
	}
	keyDomain, _ := RuntimeDomainFromValue(contextSetsComposite(t, "Set", keys...))
	values := contextSetsComposite(t, "Set", contextSetsInteger(t, "10"), contextSetsInteger(t, "20"))
	valueDomain, _ := RuntimeDomainFromValue(values)
	product, err := NewRuntimeProduct([]RuntimeDomain{keyDomain, valueDomain})
	if err != nil {
		t.Fatal(err)
	}
	compact := contextSetsDomain(t, product)
	materialized, err := product.Materialize(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var checkedHashes, priorHashes RuntimeObjectHashState
	for range 2 {
		got, err := EvalSetToMapBuiltinContext(ctx, []RuntimeValue{compact}, &checkedHashes)
		if err != nil {
			t.Fatal(err)
		}
		want, err := EvalSetToMapBuiltin(ctx, []RuntimeValue{materialized}, &priorHashes)
		if err != nil || checkedHashes.Counter() != priorHashes.Counter() {
			t.Fatal("setToMap object hashes", checkedHashes.Counter(), priorHashes.Counter(), err)
		}
		contextSetsCheckEntryOrder(t, got, want)
	}
	interval := contextSetsInterval(t, "0", "1")
	concrete, err := runtimeToSetContext(ctx, interval)
	if err != nil {
		t.Fatal(err)
	}
	length := contextSetsInteger(t, "3")
	for range 2 {
		got, err := EvalAllListsBuiltinContext(ctx, "allListsUpTo", []RuntimeValue{interval, length}, &checkedHashes)
		if err != nil {
			t.Fatal(err)
		}
		want, err := EvalAllListsBuiltin(ctx, "allListsUpTo", []RuntimeValue{concrete, length}, &priorHashes)
		if err != nil || checkedHashes.Counter() != priorHashes.Counter() {
			t.Fatal("allListsUpTo object hashes", checkedHashes.Counter(), priorHashes.Counter(), err)
		}
		contextSetsCheckEntryOrder(t, got, want)
	}
}

func TestContextSetValidationCancellationAndPrecedence(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	one := contextSetsInteger(t, "1")
	interval := contextSetsInterval(t, "1", "2")
	integerDomain, _ := NewRuntimeInfiniteSet("Int")
	integers := contextSetsDomain(t, integerDomain)
	_, err := EvalSetHigherBuiltinContext(ctx, "exists", []RuntimeValue{integers, one})
	contextSetsCheckDiagnostic(t, err, "QNT501", "Expected a lambda value")
	_, err = EvalSetHigherBuiltinContext(ctx, "filter", []RuntimeValue{integers, one})
	var exception RuntimeDomainException
	if !errors.As(err, &exception) || exception.Message != "Infinite set Int is non-enumerable" {
		t.Fatal("filter arrow was resolved before receiver conversion", err)
	}
	callback := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return one, nil })
	for _, call := range []struct {
		op   string
		args []RuntimeValue
	}{{"unknown", nil}, {"map", nil}, {"map", []RuntimeValue{{}, callback}}, {"map", []RuntimeValue{one, callback}}, {"filter", []RuntimeValue{interval, callback}}, {"forall", []RuntimeValue{interval, callback}}, {"fold", []RuntimeValue{interval, one, one}}} {
		if _, err := EvalSetHigherBuiltinContext(ctx, call.op, call.args); err == nil {
			t.Fatal("invalid higher call accepted", call.op)
		}
	}
	for _, args := range [][]RuntimeValue{nil, {RuntimeValue{}}, {one}, {interval}} {
		if _, err := EvalSetToMapBuiltinContext(ctx, args, nil); err == nil {
			t.Fatal("invalid conversion accepted")
		}
	}
	for _, args := range [][]RuntimeValue{nil, {interval, RuntimeValue{}}, {one, one}, {interval, interval}} {
		if _, err := EvalAllListsBuiltinContext(ctx, "allListsUpTo", args, &RuntimeObjectHashState{}); err == nil {
			t.Fatal("invalid allListsUpTo accepted")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, operation := range []func() error{
		func() error { _, err := EvalSetBuiltinContext(canceled, "size", nil); return err },
		func() error { _, err := EvalSetHigherBuiltinContext(canceled, "map", nil); return err },
		func() error { _, err := EvalSetToMapBuiltinContext(canceled, nil, nil); return err },
		func() error { _, err := EvalAllListsBuiltinContext(canceled, "allListsUpTo", nil, nil); return err },
	} {
		if err := operation(); !errors.Is(err, context.Canceled) {
			t.Fatal("operation ignored cancellation", err)
		}
	}
}
