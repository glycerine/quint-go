package quint

import (
	"context"
	"errors"
	"testing"
)

func symbolicInt(t *testing.T, n string) RuntimeValue {
	t.Helper()
	i, err := ParseInteger(n)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewRuntimeValue(Expr{Kind: "int", Int: i})
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func symbolicValue(t *testing.T, d RuntimeDomain) RuntimeValue {
	t.Helper()
	v, err := RuntimeValueFromDomain(d)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func symbolicComposite(t *testing.T, op string, args ...RuntimeValue) RuntimeValue {
	t.Helper()
	v, err := NewRuntimeCompositeContext(context.Background(), op, args)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestCompactRuntimeValueRetainsDomains(t *testing.T) {
	ctx := context.Background()
	huge := symbolicInt(t, "1000000000000000000000000000000")
	v, err := EvalSymbolicBuiltin(ctx, "to", []RuntimeValue{symbolicInt(t, "1"), huge})
	if err != nil {
		t.Fatal(err)
	}
	if !v.Valid() || v.domain == nil || !v.containsDomain || len(v.Expression().Args) != 2 {
		t.Fatal("interval eagerly materialized")
	}
	if v.String() != "1.to(1000000000000000000000000000000)" {
		t.Logf("compact display %s", v.String())
	}
	clone := v.Expression()
	clone.Args[0].Int = Integer{}
	if v.Expression().Args[0].Int.String() != "1" {
		t.Fatal("expression mutation escaped")
	}
	if _, err := SerializeRuntimeValue(v, nil); err == nil {
		t.Fatal("unchecked serialization accepted symbolic enumeration")
	}
	if _, err := runtimeNormalForm(v); err == nil {
		t.Fatal("unchecked normalization accepted symbolic enumeration")
	}
	if !v.Equal(v) || v.Hash() != v.Hash() {
		t.Fatal("compact structural identity unstable")
	}
	copied, err := RuntimeDomainFromValue(v)
	if err != nil || copied.kind != runtimeIntervalDomain {
		t.Fatalf("unwrap: %v", err)
	}
	if _, err := RuntimeValueFromDomain(RuntimeDomain{}); err == nil {
		t.Fatal("invalid domain accepted")
	}
}

func TestCompactRuntimeCollectionsCheckedBoundary(t *testing.T) {
	ctx := context.Background()
	one, two := symbolicInt(t, "1"), symbolicInt(t, "2")
	interval := symbolicValue(t, NewRuntimeInterval(one.expression.Int, two.expression.Int))
	explicit := symbolicComposite(t, "Set", one, two)
	list := symbolicComposite(t, "List", interval)
	concrete := symbolicComposite(t, "List", explicit)
	if list.Equal(concrete) {
		t.Fatal("context-free identity enumerated domain")
	}
	equal, err := list.EqualContext(ctx, concrete)
	if err != nil || !equal {
		t.Fatalf("extensional equality %v %v", equal, err)
	}
	h1, err := list.HashContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := concrete.HashContext(ctx)
	if err != nil || h1 != h2 {
		t.Fatalf("extensional hashes disagree %v", err)
	}
	set := symbolicComposite(t, "Set", interval, explicit)
	entries, err := runtimeEntries(set)
	if err != nil || len(entries) != 1 || set.containsDomain {
		t.Fatalf("normalized set keys: %d %v", len(entries), err)
	}
	pair := symbolicComposite(t, "Tup", interval, interval)
	m := symbolicComposite(t, "Map", pair)
	entries, err = runtimeEntries(m)
	if err != nil || len(entries) != 1 || entries[0].Key.containsDomain || !sameRuntimeObject(entries[0].Value, interval) {
		t.Fatalf("map key/value normalization %v", err)
	}
	if _, err := m.ExpressionContext(ctx); err != nil {
		t.Fatal(err)
	}
	label, _ := NewRuntimeValue(Expr{Kind: "str", Str: "D"})
	variant := symbolicComposite(t, "variant", label, interval)
	variantSet, err := NewRuntimeCompositeContext(ctx, "Set", []RuntimeValue{variant})
	if err != nil {
		t.Fatalf("retained-domain variant key: %v", err)
	}
	variantEntries, err := runtimeEntries(variantSet)
	if err != nil || len(variantEntries) != 1 || !sameRuntimeObject(variantEntries[0].Key, variant) {
		t.Fatalf("variant key payload identity: %v", err)
	}
	normalized, err := runtimeNormalFormContext(ctx, variant)
	if err != nil || !sameRuntimeObject(normalized, variant) {
		t.Fatalf("variant normalform lost payload %v", err)
	}
	state, err := newRuntimeComposite("Rec", []RuntimeValue{label, interval})
	if err != nil || !state.containsDomain {
		t.Fatalf("state record cannot retain domain: %v", err)
	}
	if _, err := state.ExpressionContext(ctx); err != nil {
		t.Fatal(err)
	}
	source := []RuntimeValue{interval}
	owned := symbolicComposite(t, "List", source...)
	source[0] = one
	children, _ := runtimeChildren(owned)
	if !sameRuntimeObject(children[0], interval) {
		t.Fatal("child slice ownership lost")
	}
}

func TestCompactDomainCheckedCancellationAndErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	one := symbolicInt(t, "1")
	interval := symbolicValue(t, NewRuntimeInterval(one.expression.Int, one.expression.Int))
	calls := []func() error{
		func() error { _, e := interval.ExpressionContext(ctx); return e },
		func() error { _, e := SerializeRuntimeValueContext(ctx, interval, nil); return e },
		func() error { _, e := interval.HashContext(ctx); return e },
		func() error { _, e := interval.EqualContext(ctx, interval); return e },
		func() error { _, e := NewRuntimeCompositeContext(ctx, "List", []RuntimeValue{interval}); return e },
		func() error { _, e := runtimeNormalFormContext(ctx, interval); return e },
		func() error { _, e := runtimeToSetContext(ctx, interval); return e },
		func() error { _, e := EvalSymbolicBuiltin(ctx, "tuples", nil); return e },
	}
	for i, call := range calls {
		if err := call(); !errors.Is(err, context.Canceled) {
			t.Fatalf("call%d cancellation %v", i, err)
		}
	}
	background := context.Background()
	infDomain, _ := NewRuntimeInfiniteSet("Int")
	inf := symbolicValue(t, infDomain)
	expr, err := inf.ExpressionContext(background)
	if err != nil || expr.Kind != "name" || expr.Name != "Int" {
		t.Fatalf("infinite display %v %v", expr, err)
	}
	if _, err := NewRuntimeCompositeContext(background, "Set", []RuntimeValue{inf}); err == nil || err.Error() != "Infinite set Int is non-enumerable" {
		t.Fatalf("infinite normalization %v", err)
	}
	product, err := EvalSymbolicBuiltin(background, "tuples", nil)
	if err != nil {
		t.Fatal(err)
	}
	expr, err = product.ExpressionContext(background)
	if err != nil || len(expr.Args) != 0 {
		t.Fatalf("nullary display %v", err)
	}
	empty := symbolicComposite(t, "Set")
	eq, err := product.EqualContext(background, empty)
	if err != nil || !eq {
		t.Fatalf("nullary extensional empty %v %v", eq, err)
	}
	a, _ := product.HashContext(background)
	b, _ := empty.HashContext(background)
	if a != b {
		t.Fatal("empty-domain hash mismatch")
	}
	for _, call := range []func() (RuntimeValue, error){func() (RuntimeValue, error) { return EvalSymbolicBuiltin(background, "unsupported", nil) }, func() (RuntimeValue, error) { return EvalSymbolicBuiltin(background, "to", nil) }, func() (RuntimeValue, error) { return EvalSymbolicBuiltin(background, "powerset", []RuntimeValue{one}) }, func() (RuntimeValue, error) {
		return NewRuntimeCompositeContext(background, "Map", []RuntimeValue{inf})
	}} {
		if _, err := call(); err == nil {
			t.Fatal("invalid symbolic operation accepted")
		}
	}
}

func TestCompactNormalizationNestedOwnership(t *testing.T) {
	ctx := context.Background()
	one, two := symbolicInt(t, "1"), symbolicInt(t, "2")
	interval := symbolicValue(t, NewRuntimeInterval(one.expression.Int, two.expression.Int))
	label, _ := NewRuntimeValue(Expr{Kind: "str", Str: "d"})
	for _, op := range []string{"List", "Tup", "Rec", "Map"} {
		var original RuntimeValue
		if op == "Rec" {
			original = symbolicComposite(t, op, label, interval)
		} else if op == "Map" {
			original = symbolicComposite(t, op, symbolicComposite(t, "Tup", label, interval))
		} else {
			original = symbolicComposite(t, op, interval)
		}
		normalized, err := runtimeNormalFormContext(ctx, original)
		if err != nil || normalized.containsDomain {
			t.Fatalf("%s normalization: %v", op, err)
		}
		equal, err := original.EqualContext(ctx, normalized)
		if err != nil || !equal {
			t.Fatalf("%s normalized differs: %v %v", op, equal, err)
		}
		if !original.containsDomain {
			t.Fatalf("%s original mutated", op)
		}
	}
	set := symbolicComposite(t, "Set", one, two)
	wrapped, err := runtimeNormalFormContext(ctx, set)
	if err != nil || wrapped.payload.entries.root != set.payload.entries.root {
		t.Fatalf("set normalform history %v", err)
	}
	explicit, err := RuntimeDomainFromValue(set)
	if err != nil {
		t.Fatal(err)
	}
	same, err := RuntimeValueFromDomain(explicit)
	if err != nil || !sameRuntimeObject(same, set) {
		t.Fatalf("explicit domain wrapper identity %v", err)
	}
	if _, err := runtimeToSetContext(ctx, one); err == nil {
		t.Fatal("nonset converted")
	}
	if _, err := runtimeNormalFormContext(ctx, RuntimeValue{}); err == nil {
		t.Fatal("invalid normalform")
	}
	cases := []struct {
		op   string
		args []RuntimeValue
	}{
		{"List", []RuntimeValue{interval, RuntimeValue{}}}, {"variant", []RuntimeValue{interval, one}}, {"variant", []RuntimeValue{interval}},
		{"Rec", []RuntimeValue{label, interval, label}}, {"Rec", []RuntimeValue{interval, one}},
		{"Map", []RuntimeValue{symbolicComposite(t, "Tup", interval)}}, {"Map", []RuntimeValue{interval}},
		{"bad", []RuntimeValue{interval}},
	}
	for _, tc := range cases {
		if _, err := NewRuntimeCompositeContext(ctx, tc.op, tc.args); err == nil {
			t.Fatalf("invalid %s accepted", tc.op)
		}
	}
}

func TestCompactConstructorsPreserveClosureChildren(t *testing.T) {
	ctx := context.Background()
	closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return symbolicInt(t, "1"), nil })
	set := symbolicComposite(t, "Set", closure)
	explicit, err := RuntimeDomainFromValue(set)
	if err != nil {
		t.Fatal(err)
	}
	inf, _ := NewRuntimeInfiniteSet("Nat")
	product, err := NewRuntimeProduct([]RuntimeDomain{explicit, inf})
	if err != nil {
		t.Fatal(err)
	}
	power, err := NewRuntimePowerset(explicit)
	if err != nil {
		t.Fatal(err)
	}
	maps, err := NewRuntimeMapSet(explicit, inf)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []RuntimeDomain{product, power, maps} {
		v := symbolicValue(t, d)
		if !v.containsClosure {
			t.Fatal("compact domain discarded closure flag")
		}
		raw, err := runtimeReferenceHashContext(ctx, v)
		if err != nil {
			t.Fatalf("raw domain hash %d %v", raw, err)
		}
	}
	for _, op := range []string{"tuples", "powerset", "setOfMaps"} {
		args := []RuntimeValue{set}
		if op == "setOfMaps" {
			args = append(args, symbolicValue(t, inf))
		}
		v, err := EvalSymbolicBuiltin(ctx, op, args)
		if err != nil || v.domain == nil {
			t.Fatalf("constructor %s %v", op, err)
		}
	}
	if _, err := EvalSymbolicBuiltin(ctx, "to", []RuntimeValue{closure, symbolicInt(t, "1")}); err == nil {
		t.Fatal("to accepted lambda")
	}
	if _, err := EvalSymbolicBuiltin(ctx, "tuples", []RuntimeValue{RuntimeValue{}}); err == nil {
		t.Fatal("tuples accepted invalid")
	}
}

func TestCompactStructuralClosureEquality(t *testing.T) {
	first := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return symbolicInt(t, "1"), nil })
	second := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return symbolicInt(t, "1"), nil })
	domain := func(value RuntimeValue) RuntimeDomain {
		d, err := RuntimeDomainFromValue(symbolicComposite(t, "Set", value))
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	a, b := domain(first), domain(second)
	for _, op := range []string{"tuples", "powerset", "setOfMaps"} {
		var left, right RuntimeDomain
		var err error
		switch op {
		case "tuples":
			left, err = NewRuntimeProduct([]RuntimeDomain{a})
			if err == nil {
				right, err = NewRuntimeProduct([]RuntimeDomain{b})
			}
		case "powerset":
			left, err = NewRuntimePowerset(a)
			if err == nil {
				right, err = NewRuntimePowerset(b)
			}
		case "setOfMaps":
			left, err = NewRuntimeMapSet(a, a)
			if err == nil {
				right, err = NewRuntimeMapSet(b, b)
			}
		}
		if err != nil {
			t.Fatal(err)
		}
		x, y := symbolicValue(t, left), symbolicValue(t, right)
		if x.Equal(y) || !x.Equal(x) {
			t.Fatalf("%s structural closure identity lost", op)
		}
	}
	args := make([]RuntimeValue, 32)
	for i := range args {
		args[i] = symbolicComposite(t, "Set", symbolicInt(t, "1"))
	}
	ctx := &contextIdentityCancelContext{Context: context.Background()}
	if _, err := EvalSymbolicBuiltin(ctx, "tuples", args); !errors.Is(err, context.Canceled) {
		t.Fatalf("constructor factor traversal cancellation %v", err)
	}
}

func TestCompactMaterializingSetOperations(t *testing.T) {
	ctx := context.Background()
	one, two := symbolicInt(t, "1"), symbolicInt(t, "2")
	interval := symbolicValue(t, NewRuntimeInterval(one.expression.Int, two.expression.Int))
	empty := symbolicValue(t, NewRuntimeInterval(two.expression.Int, one.expression.Int))
	power, err := NewRuntimePowerset(*interval.domain)
	if err != nil {
		t.Fatal(err)
	}
	flat, err := EvalSetBuiltinContext(ctx, "flatten", []RuntimeValue{symbolicValue(t, power)})
	if err != nil {
		t.Fatal(err)
	}
	want := symbolicComposite(t, "Set", one, two)
	if equal, err := flat.EqualContext(ctx, want); err != nil || !equal {
		t.Fatalf("flatten powerset %v %v", equal, err)
	}
	for _, operands := range [][]RuntimeValue{{empty, interval}, {interval, empty}, {interval, interval}} {
		got, err := EvalSetBuiltinContext(ctx, "union", operands)
		if err != nil || !got.Equal(want) {
			t.Fatalf("union empty/materialized %v", err)
		}
	}
	if _, err := EvalSetBuiltinContext(ctx, "flatten", []RuntimeValue{interval}); err == nil {
		t.Fatal("flatten scalar domain elements accepted")
	}
	product, err := NewRuntimeProduct([]RuntimeDomain{*interval.domain})
	if err != nil {
		t.Fatal(err)
	}
	maps, err := NewRuntimeMapSet(*interval.domain, *interval.domain)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []RuntimeDomain{product, power, maps} {
		v := symbolicValue(t, d)
		entries := []runtimeReferenceEntry{{Key: v, Value: v}, {Key: v, Value: v}}
		m, err := runtimeRawMapFromEntriesContext(ctx, entries)
		if err != nil || m.Len() != 1 {
			t.Fatalf("uniform raw compact %d %v", m.Len(), err)
		}
	}
}
