package quint

import (
	"context"
	"errors"
	"testing"
)

func TestMapSetByOrderCancellationAndOwnership(t *testing.T) {
	e, _ := ParseExpression("Map((1,10))")
	m, _ := NewRuntimeValue(e)
	n, _ := ParseInteger("1")
	key, _ := NewRuntimeValue(Expr{Kind: "int", Int: n})
	calls := 0
	callback := newRuntimeClosure(1, func(_ context.Context, args []RuntimeValue) (RuntimeValue, error) {
		calls++
		return EvalIntegerBuiltin("iadd", []RuntimeValue{args[0], key})
	})
	result, err := EvalMapSetBy(context.Background(), []RuntimeValue{m, key, callback})
	if err != nil || calls != 1 || result.String() != "Map(Tup(1, 11))" || m.String() != "Map(Tup(1, 10))" {
		t.Fatal(result.String(), m.String(), err, calls)
	}
	missing, _ := NewRuntimeValue(Expr{Kind: "str", Str: "missing"})
	calls = 0
	if _, err = EvalMapSetBy(context.Background(), []RuntimeValue{m, missing, callback}); err == nil || calls != 0 {
		t.Fatal(err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = EvalMapSetBy(ctx, []RuntimeValue{m, key, callback}); !errors.Is(err, context.Canceled) || calls != 0 {
		t.Fatal(err, calls)
	}
	for _, args := range [][]RuntimeValue{nil, {m, key}, {m, key, {}}, {key, key, callback}} {
		if _, err := EvalMapSetBy(context.Background(), args); err == nil {
			t.Fatal("invalid call accepted")
		}
	}
	failure := Diagnostic{Code: "QNT508", Message: "callback failure"}
	bad := newRuntimeClosure(1, func(context.Context, []RuntimeValue) (RuntimeValue, error) { return RuntimeValue{}, failure })
	_, err = EvalMapSetBy(context.Background(), []RuntimeValue{m, key, bad})
	var d Diagnostic
	if !errors.As(err, &d) || d.Code != failure.Code || m.String() != "Map(Tup(1, 10))" {
		t.Fatal(err)
	}
}
