package quint

import (
	"context"
	"fmt"
	"reflect"
	"testing"
)

// Retain the previous concrete List/Tup construction as an independent oracle.
func runtimeSequenceNormalizeOracle(op string, args []RuntimeValue) (RuntimeValue, error) {
	if runtimeHasDomain(args) {
		return NewRuntimeCompositeContext(context.Background(), op, args)
	}
	source := Expr{Kind: "app", Opcode: op, Args: make([]Expr, len(args))}
	containsClosure := false
	for i, arg := range args {
		if !arg.Valid() {
			return RuntimeValue{}, Diagnostic{Code: "QNT501", Message: "Invalid runtime value"}
		}
		source.Args[i] = arg.source
		containsClosure = containsClosure || arg.containsClosure
	}
	return runtimeAssemble(source, &runtimePayload{children: append([]RuntimeValue{}, args...), containsClosure: containsClosure})
}

func runtimeSequenceCacheValues(t testing.TB, heavy bool) []RuntimeValue {
	t.Helper()
	app, integer := normalizePreallocApp, normalizePreallocInt
	str := func(value string) Expr { return Expr{Kind: "str", Str: value} }
	values := []Expr{integer(1), Expr{Kind: "bool", Bool: true}, str("é"), app("List"), app("Tup")}
	set := app("Set", str("é"), str("e\u0301"), str("a"), str("z"), integer(7), integer(1), integer(3), integer(5), integer(2), integer(4))
	record := app("Rec", str("\ue000"), set, str("\U00010000"), integer(2), str("a"), integer(3), str("a"), integer(4))
	pairCount := 9
	if heavy {
		pairCount = 12
	}
	pairs := make([]Expr, pairCount)
	for i := range pairs {
		// These keys collide under the reference's 32-bit integer hash.
		key, _ := ParseInteger(fmt.Sprint(int64(i) * 4294967296))
		value := integer(i)
		if heavy {
			value = app("List", set, record)
		}
		pairs[i] = app("Tup", Expr{Kind: "int", Int: key}, value)
	}
	mapping := app("Map", pairs...)
	values = append(values, set, record, mapping, app("variant", str("Some"), mapping), app("List", mapping, set, record))
	id, _ := ParseID("9007199254740993123456789")
	values = append(values, Expr{ID: id, Kind: "str", Str: "retained source metadata"})
	result := make([]RuntimeValue, len(values))
	for i, expression := range values {
		var err error
		result[i], err = NewRuntimeValue(expression)
		if err != nil {
			t.Fatal(err)
		}
	}
	return result
}

func assertRuntimeSequenceEquivalent(t *testing.T, got, want RuntimeValue, args []RuntimeValue) {
	t.Helper()
	if !reflect.DeepEqual(got.expression, want.expression) || !reflect.DeepEqual(got.source, want.source) || got.identity != want.identity || got.Hash() != want.Hash() || got.containsClosure != want.containsClosure || got.containsDomain != want.containsDomain {
		t.Fatal("cached/source/identity representation changed")
	}
	if got.allocation == nil || got.allocation == want.allocation || got.payload == nil || !reflect.DeepEqual(got.payload.children, want.payload.children) || (got.payload.children == nil) != (want.payload.children == nil) {
		t.Fatal("sequence wrapper or payload ownership changed")
	}
	for i, child := range got.payload.children {
		if !sameRuntimeObject(child, args[i]) || child.payload != args[i].payload || child.domain != args[i].domain {
			t.Fatal("constructor rebuilt a retained child")
		}
	}
	gotHash, gotErr := runtimeReferenceHash(got)
	wantHash, wantErr := runtimeReferenceHash(want)
	if gotHash != wantHash || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
		t.Fatal("reference hash changed", gotErr, wantErr)
	}
	for _, initial := range []uint32{0, 7, 0x3ffffffd} {
		gotState, wantState := &RuntimeObjectHashState{}, &RuntimeObjectHashState{}
		_ = gotState.Restore(initial)
		_ = wantState.Restore(initial)
		for repeat := 0; repeat < 2; repeat++ {
			actual, e1 := SerializeRuntimeValueContext(context.Background(), got, gotState)
			expected, e2 := SerializeRuntimeValueContext(context.Background(), want, wantState)
			if fmt.Sprint(e1) != fmt.Sprint(e2) || !reflect.DeepEqual(actual, expected) || gotState.Counter() != wantState.Counter() {
				t.Fatal("serialization/HAMT/UID history changed", initial, repeat, gotState.Counter(), wantState.Counter(), e1, e2)
			}
		}
	}
}

func TestRuntimeSequenceCachedViewsEquivalence(t *testing.T) {
	t.Parallel()
	values := runtimeSequenceCacheValues(t, false)
	for _, op := range []string{"List", "Tup"} {
		for _, tc := range []struct {
			name string
			args []RuntimeValue
		}{
			{"nil", nil},
			{"empty", values[:0]},
			{"one", values[:1]},
			{"scalars", values[:3]},
			{"nested", values},
		} {
			t.Run(op+"/"+tc.name, func(t *testing.T) {
				t.Parallel()
				args := tc.args
				want, err := runtimeSequenceNormalizeOracle(op, args)
				if err != nil {
					t.Fatal(err)
				}
				got, err := newRuntimeComposite(op, args)
				if err != nil {
					t.Fatal(err)
				}
				assertRuntimeSequenceEquivalent(t, got, want, args)
			})
		}
	}
}

func TestRuntimeSequenceCachedViewsOwnership(t *testing.T) {
	t.Parallel()
	child := runtimeSequenceCacheValues(t, false)[9]
	args := []RuntimeValue{child, child}
	childExpression, childSource := copyRuntimeExpression(child.expression), copyRuntimeExpression(child.source)
	for _, op := range []string{"List", "Tup"} {
		callerArgs := append([]RuntimeValue{}, args...)
		got, err := newRuntimeComposite(op, callerArgs)
		if err != nil {
			t.Fatal(err)
		}
		callerArgs[0] = RuntimeValue{}
		if !sameRuntimeObject(got.payload.children[0], child) {
			t.Fatal("constructor retained caller child slice")
		}
		want, err := runtimeSequenceNormalizeOracle(op, args)
		if err != nil {
			t.Fatal(err)
		}
		view := got.Expression()
		view.Args[0].Args[0].Args = nil
		if !reflect.DeepEqual(got.expression, want.expression) {
			t.Fatal("public expression exposed cached storage")
		}
		got.expression.Args[0].Args[0].Args = nil
		if !reflect.DeepEqual(got.expression.Args[1], want.expression.Args[1]) || !reflect.DeepEqual(got.source, want.source) || !reflect.DeepEqual(child.expression, childExpression) {
			t.Fatal("cached children/siblings/source share mutable expression views")
		}
		got.source.Args[0].Args = nil
		if !reflect.DeepEqual(got.source.Args[1], want.source.Args[1]) || !reflect.DeepEqual(child.source, childSource) {
			t.Fatal("source copies share mutable trees")
		}
		children, err := runtimeChildren(got)
		if err != nil {
			t.Fatal(err)
		}
		children[0] = RuntimeValue{}
		if !sameRuntimeObject(got.payload.children[0], child) {
			t.Fatal("child slice exposed payload storage")
		}
	}
}

func TestRuntimeSequenceCachedViewsFallbacks(t *testing.T) {
	t.Parallel()
	closure := newRuntimeClosure(0, func(context.Context, []RuntimeValue) (RuntimeValue, error) {
		return NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	})
	domain, err := RuntimeValueFromDomain(NewRuntimeInterval(normalizePreallocInt(1).Int, normalizePreallocInt(3).Int))
	if err != nil {
		t.Fatal(err)
	}
	transient := RuntimeValue{expression: normalizePreallocApp("Tup"), identity: "serializer transient", allocation: &runtimeValueAllocation{}}
	concrete, err := NewRuntimeValue(normalizePreallocInt(1))
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range []string{"List", "Tup"} {
		for _, args := range [][]RuntimeValue{{{}}, {concrete, {}}, {transient}, {closure}, {concrete, closure}, {transient, closure}, {domain}, {domain, {}}} {
			want, wantErr := runtimeSequenceNormalizeOracle(op, args)
			got, gotErr := newRuntimeComposite(op, args)
			if (gotErr == nil) != (wantErr == nil) || fmt.Sprint(gotErr) != fmt.Sprint(wantErr) {
				t.Fatal("fallback changed error", op, gotErr, wantErr)
			}
			if gotErr == nil {
				assertRuntimeSequenceEquivalent(t, got, want, args)
			}
		}
	}
}

func TestRuntimeSequenceCachedViewsMapObjectHistory(t *testing.T) {
	t.Parallel()
	args := runtimeSequenceCacheValues(t, false)[5:]
	key, _ := NewRuntimeValue(Expr{Kind: "bool", Bool: true})
	for _, op := range []string{"List", "Tup"} {
		stored, err := newRuntimeComposite(op, args)
		if err != nil {
			t.Fatal(err)
		}
		fresh, err := newRuntimeComposite(op, args)
		if err != nil || !stored.Equal(fresh) || sameRuntimeObject(stored, fresh) {
			t.Fatal("equal constructor calls must retain distinct wrapper identities", err)
		}
		pair, err := newRuntimeComposite("Tup", []RuntimeValue{key, stored})
		if err != nil {
			t.Fatal(err)
		}
		mapping, err := newRuntimeComposite("Map", []RuntimeValue{pair})
		if err != nil {
			t.Fatal(err)
		}
		before, _ := runtimeMap(mapping)
		unchanged, err := EvalMapBuiltin("put", []RuntimeValue{mapping, key, stored})
		if err != nil {
			t.Fatal(err)
		}
		same, _ := runtimeMap(unchanged)
		if same.root != before.root {
			t.Fatal("same stored object changed persistent map history")
		}
		changed, err := EvalMapBuiltin("put", []RuntimeValue{mapping, key, fresh})
		if err != nil {
			t.Fatal(err)
		}
		after, _ := runtimeMap(changed)
		retained, e1 := EvalMapBuiltin("get", []RuntimeValue{mapping, key})
		updated, e2 := EvalMapBuiltin("get", []RuntimeValue{changed, key})
		if after.root == before.root || e1 != nil || e2 != nil || !sameRuntimeObject(retained, stored) || !sameRuntimeObject(updated, fresh) {
			t.Fatal("new equal wrapper lost map replacement identity or mutated the parent", e1, e2)
		}
	}
}
