package quint

import (
	"context"
	"testing"
)

func TestUpstreamPayloadAccess(t *testing.T) {
	testPureEvaluationFixtures(t, "runtime_payload_access.json")
}

func TestPayloadAccessRetainsChildObjects(t *testing.T) {
	scalar := func(e Expr) RuntimeValue {
		t.Helper()
		v, err := NewRuntimeValue(e)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	integer := func(s string) RuntimeValue { n, _ := ParseInteger(s); return scalar(Expr{Kind: "int", Int: n}) }
	composite := func(op string, children ...RuntimeValue) RuntimeValue {
		t.Helper()
		v, err := newRuntimeComposite(op, children)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	invoke := func(op string, args ...RuntimeValue) RuntimeValue {
		t.Helper()
		v, err := EvalSequenceBuiltin(op, args)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	requireSame := func(label string, got, want RuntimeValue) {
		t.Helper()
		if !sameRuntimeObject(got, want) {
			t.Fatal(label, "reconstructed child", got.String(), want.String())
		}
	}
	zero, one := integer("0"), integer("1")
	child := composite("Map", composite("Tup", one, integer("10")))
	list := composite("List", child)
	requireSame("head", invoke("head", list), child)
	requireSame("nth", invoke("nth", list, zero), child)
	requireSame("item", invoke("item", composite("Tup", child), one), child)
	requireSame("tail", invoke("head", invoke("tail", composite("List", zero, child))), child)
	requireSame("append", invoke("head", invoke("append", composite("List"), child)), child)
	requireSame("concat", invoke("head", invoke("concat", composite("List"), list)), child)
	requireSame("replaceAt", invoke("head", invoke("replaceAt", composite("List", zero), zero, child)), child)
	requireSame("slice", invoke("head", invoke("slice", list, zero, one)), child)
	field := scalar(Expr{Kind: "str", Str: "x"})
	record := composite("Rec", field, child)
	requireSame("field", invoke("field", record, field), child)
	requireSame("with", invoke("field", invoke("with", composite("Rec", field, zero), field, child), field), child)
	requireSame("duplicate field", invoke("field", composite("Rec", field, zero, field, child), field), child)
	owned, err := runtimeChildren(list)
	if err != nil {
		t.Fatal(err)
	}
	owned[0] = zero
	requireSame("copied child slice", invoke("head", list), child)
	requireSame("old list unchanged", invoke("head", list), child)
	for _, op := range []string{"foldl", "foldr"} {
		calls := 0
		callback := newRuntimeClosure(2, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
			calls++
			index := 1
			if op == "foldr" {
				index = 0
			}
			requireSame("fold callback", args[index], child)
			return args[index], nil
		})
		result, err := EvalListHigherBuiltin(context.Background(), op, []RuntimeValue{list, zero, callback})
		if err != nil || calls != 1 {
			t.Fatal(op, calls, err)
		}
		requireSame(op, result, child)
	}
	predicate := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		requireSame("select callback", args[0], child)
		return scalar(Expr{Kind: "bool", Bool: true}), nil
	})
	selected, err := EvalListHigherBuiltin(context.Background(), "select", []RuntimeValue{list, predicate})
	if err != nil {
		t.Fatal(err)
	}
	requireSame("select", invoke("head", selected), child)
	set := composite("Set", child)
	entries, err := runtimeEntries(set)
	if err != nil {
		t.Fatal(err)
	}
	singleton, err := EvalSetConversionBuiltin("getOnlyElement", []RuntimeValue{set})
	if err != nil {
		t.Fatal(err)
	}
	requireSame("singleton", singleton, entries[0].Value)
	outer := composite("Map", composite("Tup", one, child))
	replacement := composite("Map", composite("Tup", one, integer("11")))
	callback := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		requireSame("setBy current", args[0], child)
		return replacement, nil
	})
	updated, err := EvalMapSetBy(context.Background(), []RuntimeValue{outer, one, callback})
	if err != nil {
		t.Fatal(err)
	}
	current, err := EvalMapBuiltin("get", []RuntimeValue{updated, one})
	if err != nil {
		t.Fatal(err)
	}
	requireSame("setBy result", current, replacement)
	current, err = EvalMapBuiltin("get", []RuntimeValue{outer, one})
	if err != nil {
		t.Fatal(err)
	}
	requireSame("setBy original", current, child)
}
